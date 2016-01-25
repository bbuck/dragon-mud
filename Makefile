SOURCE_FILES := $(shell go list ./... | grep -v /vendor/)

test:
	ginkgo -skipPackage=vendor -r

install:
	go install github.com/bbuck/dragon-mud/...

coveralls: get-coveralls-reqs
	goveralls -service=travis-ci $(SOURCE_FILES)
	
bootstrap: get-glide get-deps
  
get-glide:
	go get github.com/Masterminds/glide
  
get-deps:
	glide install
	go get github.com/onsi/ginkgo/ginkgo
  
get-coveralls-reqs:
	go get github.com/axw/gocov/gocov
	go get golang.org/x/tools/cmd/cover
	go get github.com/mattn/goveralls
  
.PHONY: test install coveralls bootstrap get-glide get-deps get-coveralls-reqs
