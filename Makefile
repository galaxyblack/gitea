DIST := dist
IMPORT := code.gitea.io/gitea

GO ?= go
SED_INPLACE := sed -i

ifeq ($(OS), Windows_NT)
	EXECUTABLE := gitea.exe
else
	EXECUTABLE := gitea
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Darwin)
		SED_INPLACE := sed -i ''
	endif
endif

BINDATA := modules/{options,public,templates}/bindata.go
GOFILES := $(shell find . -name "*.go" -type f ! -path "./vendor/*" ! -path "*/bindata.go")
GOFMT ?= gofmt -s

GOFLAGS := -i -v
EXTRA_GOFLAGS ?=

LDFLAGS := -X "main.Version=$(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')" -X "main.Tags=$(TAGS)"

PACKAGES ?= $(filter-out code.gitea.io/gitea/integrations,$(shell $(GO) list ./... | grep -v /vendor/))
SOURCES ?= $(shell find . -name "*.go" -type f)

TAGS ?=

TMPDIR := $(shell mktemp -d 2>/dev/null || mktemp -d -t 'gitea-temp')

TEST_MYSQL_HOST ?= mysql:3306
TEST_MYSQL_DBNAME ?= testgitea
TEST_MYSQL_USERNAME ?= root
TEST_MYSQL_PASSWORD ?=
TEST_PGSQL_HOST ?= pgsql:5432
TEST_PGSQL_DBNAME ?= testgitea
TEST_PGSQL_USERNAME ?= postgres
TEST_PGSQL_PASSWORD ?= postgres

EXECUTABLE := gitea

ifneq ($(DRONE_TAG),)
	VERSION ?= $(subst v,,$(DRONE_TAG))
else
	ifneq ($(DRONE_BRANCH),)
		VERSION ?= $(subst release/v,,$(DRONE_BRANCH))
	else
		VERSION ?= master
	endif
endif

.PHONY: all
all: build

.PHONY: clean
clean:
	$(GO) clean -i ./...
	rm -rf $(EXECUTABLE) $(DIST) $(BINDATA) \
		integrations*.test \
		integrations/gitea-integration-pgsql/ integrations/gitea-integration-mysql/ integrations/gitea-integration-sqlite/ \
		integrations/indexers-mysql/ integrations/indexers-pgsql integrations/indexers-sqlite \
		integrations/mysql.ini integrations/pgsql.ini

.PHONY: fmt
fmt:
	$(GOFMT) -w $(GOFILES)

.PHONY: vet
vet:
	$(GO) vet $(PACKAGES)

.PHONY: generate
generate:
	@hash go-bindata > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/jteeuwen/go-bindata/...; \
	fi
	$(GO) generate $(PACKAGES)

.PHONY: generate-swagger
generate-swagger:
	@hash swagger > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/go-swagger/go-swagger/cmd/swagger; \
	fi
	swagger generate spec -o ./public/swagger.v1.json

.PHONY: errcheck
errcheck:
	@hash errcheck > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/kisielk/errcheck; \
	fi
	errcheck $(PACKAGES)

.PHONY: lint
lint:
	@hash golint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/golang/lint/golint; \
	fi
	for PKG in $(PACKAGES); do golint -set_exit_status $$PKG || exit 1; done;

.PHONY: misspell-check
misspell-check:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -error -i unknwon $(GOFILES)

.PHONY: misspell
misspell:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -w -i unknwon $(GOFILES)

.PHONY: fmt-check
fmt-check:
	# get all go files and run go fmt on them
	@diff=$$($(GOFMT) -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: test
test:
	$(GO) test -tags=sqlite $(PACKAGES)

.PHONY: coverage
coverage:
	@hash gocovmerge > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/wadey/gocovmerge; \
	fi
	gocovmerge integration.coverage.out $(shell find . -type f -name "coverage.out") > coverage.all;\

.PHONY: unit-test-coverage
unit-test-coverage:
	for PKG in $(PACKAGES); do $(GO) test -tags=sqlite -cover -coverprofile $$GOPATH/src/$$PKG/coverage.out $$PKG || exit 1; done;

.PHONY: test-vendor
test-vendor:
	@hash govendor > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/kardianos/govendor; \
	fi
	govendor list +unused | tee "$(TMPDIR)/wc-gitea-unused"
	[ $$(cat "$(TMPDIR)/wc-gitea-unused" | wc -l) -eq 0 ] || echo "Warning: /!\\ Some vendor are not used /!\\"

	govendor list +outside | tee "$(TMPDIR)/wc-gitea-outside"
	[ $$(cat "$(TMPDIR)/wc-gitea-outside" | wc -l) -eq 0 ] || exit 1

	govendor status || exit 1

.PHONY: test-sqlite
test-sqlite: integrations.sqlite.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test

generate-ini:
	sed -e 's|{{TEST_MYSQL_HOST}}|${TEST_MYSQL_HOST}|g' \
		-e 's|{{TEST_MYSQL_DBNAME}}|${TEST_MYSQL_DBNAME}|g' \
		-e 's|{{TEST_MYSQL_USERNAME}}|${TEST_MYSQL_USERNAME}|g' \
		-e 's|{{TEST_MYSQL_PASSWORD}}|${TEST_MYSQL_PASSWORD}|g' \
			integrations/mysql.ini.tmpl > integrations/mysql.ini
	sed -e 's|{{TEST_PGSQL_HOST}}|${TEST_PGSQL_HOST}|g' \
		-e 's|{{TEST_PGSQL_DBNAME}}|${TEST_PGSQL_DBNAME}|g' \
		-e 's|{{TEST_PGSQL_USERNAME}}|${TEST_PGSQL_USERNAME}|g' \
		-e 's|{{TEST_PGSQL_PASSWORD}}|${TEST_PGSQL_PASSWORD}|g' \
			integrations/pgsql.ini.tmpl > integrations/pgsql.ini

.PHONY: test-mysql
test-mysql: integrations.test generate-ini
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.test

.PHONY: test-pgsql
test-pgsql: integrations.test generate-ini
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./integrations.test

.PHONY: bench-sqlite
bench-sqlite: integrations.sqlite.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-mysql
bench-mysql: integrations.test generate-ini
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-pgsql
bench-pgsql: integrations.test generate-ini
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./integrations.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .


.PHONY: integration-test-coverage
integration-test-coverage: integrations.cover.test generate-ini
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.cover.test -test.coverprofile=integration.coverage.out

integrations.test: $(SOURCES)
	$(GO) test -c code.gitea.io/gitea/integrations -o integrations.test

integrations.sqlite.test: $(SOURCES)
	$(GO) test -c code.gitea.io/gitea/integrations -o integrations.sqlite.test -tags 'sqlite'

integrations.cover.test: $(SOURCES)
	$(GO) test -c code.gitea.io/gitea/integrations -coverpkg $(shell echo $(PACKAGES) | tr ' ' ',') -o integrations.cover.test

.PHONY: check
check: test

.PHONY: install
install: $(wildcard *.go)
	$(GO) install -v -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)'

.PHONY: build
build: $(EXECUTABLE)

$(EXECUTABLE): $(SOURCES)
	$(GO) build $(GOFLAGS) $(EXTRA_GOFLAGS) -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -o $@

.PHONY: release
release: release-dirs release-windows release-linux release-darwin release-copy release-check

.PHONY: release-dirs
release-dirs:
	mkdir -p $(DIST)/binaries $(DIST)/release

.PHONY: release-windows
release-windows:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/karalabe/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'windows/*' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	mv /build/* $(DIST)/binaries
endif

.PHONY: release-linux
release-linux:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/karalabe/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'linux/*' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	mv /build/* $(DIST)/binaries
endif

.PHONY: release-darwin
release-darwin:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/karalabe/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo $(TAGS)' -ldflags '$(LDFLAGS)' -targets 'darwin/*' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	mv /build/* $(DIST)/binaries
endif

.PHONY: release-copy
release-copy:
	$(foreach file,$(wildcard $(DIST)/binaries/$(EXECUTABLE)-*),cp $(file) $(DIST)/release/$(notdir $(file));)

.PHONY: release-check
release-check:
	cd $(DIST)/release; $(foreach file,$(wildcard $(DIST)/release/$(EXECUTABLE)-*),sha256sum $(notdir $(file)) > $(notdir $(file)).sha256;)

.PHONY: javascripts
javascripts: public/js/index.js

.IGNORE: public/js/index.js
public/js/index.js: $(JAVASCRIPTS)
	cat $< >| $@

.PHONY: stylesheets-check
stylesheets-check: generate-stylesheets
	@diff=$$(git diff public/css/index.css); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make generate-stylesheets' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: generate-stylesheets
generate-stylesheets:
	node_modules/.bin/lessc --no-ie-compat --clean-css public/less/index.less public/css/index.css

.PHONY: swagger-ui
swagger-ui:
	rm -Rf public/vendor/assets/swagger-ui
	git clone --depth=10 -b v3.3.2 --single-branch https://github.com/swagger-api/swagger-ui.git $(TMPDIR)/swagger-ui
	mv $(TMPDIR)/swagger-ui/dist public/vendor/assets/swagger-ui
	rm -Rf $(TMPDIR)/swagger-ui
	$(SED_INPLACE) "s;http://petstore.swagger.io/v2/swagger.json;../../../swagger.v1.json;g" public/vendor/assets/swagger-ui/index.html

.PHONY: update-translations
update-translations:
	mkdir -p ./translations
	cd ./translations && curl -L https://crowdin.com/download/project/gitea.zip > gitea.zip && unzip gitea.zip
	rm ./translations/gitea.zip
	$(SED_INPLACE) -e 's/="/=/g' -e 's/"$$//g' ./translations/*.ini
	$(SED_INPLACE) -e 's/\\"/"/g' ./translations/*.ini
	mv ./translations/*.ini ./options/locale/
	rmdir ./translations

.PHONY: generate-images
generate-images:
	mkdir -p $(TMPDIR)/images
	inkscape -f $(PWD)/assets/logo.svg -w 880 -h 880 -e $(PWD)/public/img/gitea-lg.png
	inkscape -f $(PWD)/assets/logo.svg -w 120 -h 120 -jC -i layer1 -e $(TMPDIR)/images/sm-1.png
	inkscape -f $(PWD)/assets/logo.svg -w 120 -h 120 -jC -i layer2 -e $(TMPDIR)/images/sm-2.png
	composite -compose atop $(TMPDIR)/images/sm-2.png $(TMPDIR)/images/sm-1.png $(PWD)/public/img/gitea-sm.png
	inkscape -f $(PWD)/assets/logo.svg -w 200 -h 200 -e $(PWD)/public/img/avatar_default.png
	inkscape -f $(PWD)/assets/logo.svg -w 180 -h 180 -e $(PWD)/public/img/favicon.png
	inkscape -f $(PWD)/assets/logo.svg -w 128 -h 128 -e $(TMPDIR)/images/128-raw.png
	inkscape -f $(PWD)/assets/logo.svg -w 64 -h 64 -e $(TMPDIR)/images/64-raw.png
	inkscape -f $(PWD)/assets/logo.svg -w 32 -h 32 -jC -i layer1 -e $(TMPDIR)/images/32-1.png
	inkscape -f $(PWD)/assets/logo.svg -w 32 -h 32 -jC -i layer2 -e $(TMPDIR)/images/32-2.png
	composite -compose atop $(TMPDIR)/images/32-2.png $(TMPDIR)/images/32-1.png $(TMPDIR)/images/32-raw.png
	inkscape -f $(PWD)/assets/logo.svg -w 16 -h 16 -jC -i layer1 -e $(TMPDIR)/images/16-raw.png
	zopflipng $(TMPDIR)/images/128-raw.png $(TMPDIR)/images/128.png
	zopflipng $(TMPDIR)/images/64-raw.png $(TMPDIR)/images/64.png
	zopflipng $(TMPDIR)/images/32-raw.png $(TMPDIR)/images/32.png
	zopflipng $(TMPDIR)/images/16-raw.png $(TMPDIR)/images/16.png
	rm -f $(TMPDIR)/images/*-*.png
	convert $(TMPDIR)/images/16.png $(TMPDIR)/images/32.png \
					$(TMPDIR)/images/64.png $(TMPDIR)/images/128.png \
					$(PWD)/public/img/favicon.ico
	rm -rf $(TMPDIR)/images
