package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
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
}

func newDBStore(dsn string) (*db, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	return &db{db: conn}, conn.Ping()
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

func (s db) tx(fn func(*sql.Tx) error) error {
	t, err := s.db.Begin()
	if err != nil {
		return err
	}

	if err := fn(t); err != nil {
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

func (s db) wait(rev int64) ([]job, error) {
	_, err := s.db.Exec(`do sleep('60 ` + waitCondition + `')`)

	// we've been killed or the connection has errored
	// throttle ourselves until we can resume operation
	if err != nil {
		time.Sleep(time.Second)
	}

	rows, err := s.db.Query(`
		select path, data, rev
		  from jobs
		 where rev > ?
		 order by rev
	`, rev)
	if err != nil {
		return nil, err
	}

	var jobs []job
	for rows.Next() {
		var j job
		err := rows.Scan(&j.path, &j.data, &j.rev)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
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
		/*
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
		*/

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

	return rev, s.notify()
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

func (s db) services(glob string) ([]Service, error) {
	data := []byte{}
	srvs := []Service{}

	rows, err := s.db.Query(`
		select data
		  from jobs
		 where path like replace(?, '*', '%')
	`, glob)

	if err != nil {
		return srvs, err
	}

	for rows.Next() {
		if err := rows.Scan(&data); err != nil {
			return srvs, err
		}

		job := new(Job)
		if err := proto.Unmarshal(data, job); err != nil {
			return srvs, err
		}

		for _, srv := range job.Services() {
			srvs = append(srvs, srv)
		}
	}

	if err := rows.Err(); err != nil {
		return srvs, err
	}

	return srvs, nil
}

func (s db) Match(glob ServiceAddress, watch WatchFunc) ([]Service, error) {
	srvs, err := s.services(glob.JobPath())
	if err != nil {
		return srvs, err
	}

	match := []Service{}
	for _, srv := range srvs {
		if glob.Match(srv.Address()) {
			match = append(match, srv)
		}
	}

	// TODO register watch @ latest revision

	return match, nil
}
