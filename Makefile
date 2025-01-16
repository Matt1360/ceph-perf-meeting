.PHONY: all
all: \
	tidy \
	go

.PHONY: go
go: $(shell ls -d cmd/* | sed 's/cmd\//go./g')

go.%: BINARY=$*
go.%: 
	@$(call go_build,${BINARY})

tidy:
	go mod tidy
	go mod vendor

define go_build
	$(eval $@_SERVICE = $(1))

	@mkdir -p bin
	@echo -e '\033[0;32mgo.${$@_SERVICE}\033[0m'
	CGO_ENABLED=0 \
	go build -v \
		-ldflags "-s -w" \
		-o ./bin/${$@_SERVICE} \
		./cmd/${$@_SERVICE}
endef
