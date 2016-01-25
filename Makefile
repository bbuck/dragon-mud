test:
	ginkgo -skipPackage=vendor -r

install:
	go install github.com/bbuck/dragon-mud/...
	
bootstrap: get-glide get-deps
  
get-glide:
	go get -u github.com/Masterminds/glide
  
get-deps:
	glide install
  
.PHONY: test install bootstrap get-glid get-deps
