test:
	ginkgo -skipPackage=vendor -r
  
install:
	go install github.com/bbuck/dragon-mud/...
