package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"code.google.com/p/goprotobuf/proto"
	_ "github.com/go-sql-driver/mysql"
)

var ErrConflict = errors.New("not overwriting a newer value")

var dsn = flag.String("mysql",
	"root:@tcp(localhost:3306)/sd_test?timeout=30s",
	"DSN of the Mysql database to connect to [user:pass@proto(addr)/schema?opts]",
)

var ddls = []string{
	`create table if not exists jobs (
		path varchar(255) binary primary key,
		data blob not null,
		rev  integer not null,

		unique index idx_revision (rev) 
	) engine=innodb`,

	`create table if not exists seq (
		id  varchar(255) binary primary key,
		cur integer not null
	) engine=innodb`,

	`insert ignore into seq (id, cur) values ('job', 0)`,
}

const waitCondition = "waiting for jobs"

type db struct {
	db *sql.DB

	m       sync.Mutex // protects below
	watches []*dbWatch
}

type dbWatch struct {
	glob     ServiceAddress
	rev      int64
	callback WatchFunc
	jobs     []*Job
}

func newDBStore(dsn string) (*db, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	db := &db{
		db:      conn,
		watches: []*dbWatch{},
	}
	go db.sync()
	return db, nil
}

func (s db) ensureSchema() error {
	for _, ddl := range ddls {
		if _, err := s.db.Exec(ddl); err != nil {
			return fmt.Errorf("failed update the DDL: %q: %v", ddl, err)
		}
	}
	return nil
}

// DESTRUCTIVE
func (s *db) resetData() error {
	if _, err := s.db.Exec("delete from jobs"); err != nil {
		return err
	}
	return nil
}

func (s db) tx(transaction func(*sql.Tx) error) error {
	t, err := s.db.Begin()
	if err != nil {
		return err
	}

	if err := transaction(t); err != nil {
		if err := t.Rollback(); err != nil {
			log.Printf("db: rollback unsuccessful: %v", err)
		}
		return err
	}

	if err := t.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *db) listen(w *dbWatch) {
	s.m.Lock()
	defer s.m.Unlock()
	s.watches = append(s.watches, w)
}

func (s db) notify() error {
	rows, err := s.db.Query(`
		select id
		  from information_schema.processlist
		 where info like '%` + waitCondition + `%'
		   and info not like '%processlist%'
	`)
	if err != nil {
		return err
	}

	for rows.Next() {
		var id int64
		err = rows.Scan(&id)
		if err != nil {
			return err
		}
		s.db.Exec(`kill query ?`, id)
	}

	return nil
}

type job struct {
	path string
	data []byte
	rev  int64
}

func byPath(jobs []*Job) map[Path]*Job {
	m := map[Path]*Job{}
	for _, job := range jobs {
		m[job.Path()] = job
	}
	return m
}

func diff(olds []*Job, updates []*Job) []Change {
	changes := []Change{}

	oldMap := byPath(olds)
	newMap := byPath(updates)

	for path, old := range oldMap {
		if updated := newMap[path]; updated != nil {
			changes = append(changes, (*Job).Diff(old, updated)...)
			delete(newMap, path)
		}
	}

	for _, add := range newMap {
		changes = append(changes, (*Job).Diff(nil, add)...)
	}

	return changes
}

func (s *db) sync() {
	for {
		// Keeps the config in the session
		err := s.tx(func(tx *sql.Tx) error {
			s.db.Exec(`set session long_query_time=61`)
			defer s.db.Exec(`set session long_query_time=DEFAULT`)
			if _, err := s.db.Exec(`do sleep('60 ` + waitCondition + `')`); err != nil {
				return err
			}
			return nil
		})

		// we've been killed or the connection has errored
		// throttle ourselves until we can resume operation
		if err != nil {
			log.Println("db error in sync:", err)
			time.Sleep(time.Second)
			continue
		}

		func() { // critical section
			s.m.Lock()
			defer s.m.Unlock()
			var keep []*dbWatch

			if len(s.watches) > 0 {
			keepers:
				for _, ws := range s.watches {
					jobs, nextRev, err := s.jobs(ws.glob.JobPath(), ws.rev)
					fmt.Println(ws.glob, ws.rev, nextRev, diff(ws.jobs, jobs))
					if err != nil {
						continue keepers
					}

					for _, ch := range diff(ws.jobs, jobs) {
						if !ws.callback(ch) {
							continue keepers
						}
					}

					// record the state we've seen up until now
					ws.jobs = jobs
					ws.rev = nextRev

					keep = append(keep, ws)
				}

				s.watches = append(s.watches[:0], keep...)
			}
		}()
	}
}

func (s db) Put(ref Ref) (Ref, error) {
	data, err := proto.Marshal(ref.Job)
	if err != nil {
		return ref, err
	}
	rev, err := s.put(ref.Job.Path().String(), data, ref.Rev)
	return Ref{Job: ref.Job, Rev: rev}, err
}

func (s db) put(path string, data []byte, oldRev int64) (int64, error) {
	var rev int64

	err := s.tx(func(tx *sql.Tx) error {
		if oldRev > 0 {
			err := tx.QueryRow(`
				select rev
				  from jobs
				 where path = ?
				   and rev > ?
					 for update`,
				path, oldRev).Scan(&rev)

			if err != nil {
				return err
			}

			if rev > oldRev {
				return ErrConflict
			}
		}

		res, err := tx.Exec(`
			update seq set cur = last_insert_id(cur+1)
			 where id = 'job'
		`)
		if err != nil {
			return err
		}

		// update rev in outer scope
		if rev, err = res.LastInsertId(); err != nil {
			return err
		}

		_, err = tx.Exec(`
			insert into jobs (path, data, rev)
		  values (?,?,?)
			on duplicate key update
			  data = values(data),
				rev  = values(rev)
		`, path, data, rev)

		return err
	})
	if err != nil {
		return 0, err
	}

	if err := s.notify(); err != nil {
		return rev, err
	}

	return rev, nil
}

func (s db) Get(path Path) (*Ref, error) {
	var rev int64
	var data []byte

	if err := s.db.QueryRow(`
		select rev, data
		  from jobs
		 where path = ?
	`, string(path)).Scan(&rev, &data); err != nil {
		return nil, err
	}

	ref := &Ref{Rev: rev}
	if len(data) == 0 { // deleted
		return ref, nil
	}

	ref.Job = new(Job)
	if err := proto.Unmarshal(data, ref.Job); err != nil {
		return nil, err
	}

	return ref, nil
}

func (s db) jobs(glob string, rev int64) ([]*Job, int64, error) {
	max := rev
	data := []byte{}
	jobs := []*Job{}

	rows, err := s.db.Query(`
		select rev, data
		  from jobs
		 where path like replace(?, '*', '%')
		   and rev > ?
		 order by path, rev
	`, glob, rev)

	if err != nil {
		return nil, max, err
	}

	for rows.Next() {
		if err := rows.Scan(&rev, &data); err != nil {
			return nil, max, err
		}

		if rev > max {
			max = rev
		}

		job := new(Job)
		if err := proto.Unmarshal(data, job); err != nil {
			return nil, max, err
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return jobs, max, err
	}

	return jobs, max, nil
}

func services(glob ServiceAddress, jobs []*Job) []Service {
	srvs := []Service{}

	for _, job := range jobs {
		for _, srv := range job.Services() {
			if glob.Match(srv.Address()) {
				srvs = append(srvs, srv)
			}
		}
	}

	return srvs
}

func (s *db) Match(glob ServiceAddress, watch WatchFunc) ([]Service, error) {
	jobs, max, err := s.jobs(glob.JobPath(), 0)
	if err != nil {
		return nil, err
	}

	if watch != nil {
		s.listen(&dbWatch{glob, max, watch, jobs})
	}

	return services(glob, jobs), nil
}
