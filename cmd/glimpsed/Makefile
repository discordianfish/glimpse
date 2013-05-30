all: job.pb.go

gen:
	go get -u code.google.com/p/goprotobuf/protoc-gen-go

todoc:
	sed -e 's/^package main$$/package glimpse/' -i '' *.go

tomain:
	sed -e 's/^package glimpse$$/package main/' -i '' *.go

%.pb.go: %.proto
	protoc --go_out=. $<
	sed -e 's/package glimpse/package main/g' -i '' $@

.PHONY: all gen
