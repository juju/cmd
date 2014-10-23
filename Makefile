default: check

check:
	go test && go test -compiler gccgo

docs:
	godoc2md github.com/juju/cmd > README.md

