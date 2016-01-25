test:
	ginkgo -skipPackage=vendor -r

install:
	go install github.com/bbuck/dragon-mud/...
	
bootstrap: get-glide get-deps get-coveralls-reqs
  
get-glide:
	go get github.com/Masterminds/glide
  
get-deps:
	glide install
  
get-coveralls-reqs:
	go get golang.org/x/tools/cmd/cover
	go get github.com/mattn/goveralls
  
.PHONY: test install bootstrap get-glid get-deps
