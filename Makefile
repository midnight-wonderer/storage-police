PROGNAME = storage-police
OUTSUFFIX = bin/$(PROGNAME)
BUILDOPTS = -trimpath
LDFLAGS = -ldflags="-s -w"

src = $(shell find . -name "*.go") go.mod go.sum

.PHONY: all clean

clean:
	rm -rf bin

all: $(OUTSUFFIX).linux-amd64 \
	$(OUTSUFFIX).linux-arm \
	$(OUTSUFFIX).linux-arm64 \
	$(OUTSUFFIX).linux-mips \
	$(OUTSUFFIX).linux-mipsle \
	$(OUTSUFFIX).darwin-amd64 \
	$(OUTSUFFIX).darwin-arm64 \
	$(OUTSUFFIX).windows-amd64.exe \
	$(OUTSUFFIX).windows-arm64.exe

$(OUTSUFFIX).%: $(src)
	GOOS=$(word 1,$(subst -, ,$*)) \
	GOARCH=$(subst .exe,,$(word 2,$(subst -, ,$*))) \
	$(if $(findstring mips,$*),GOMIPS=softfloat,) \
	$(if $(findstring windows-arm,$*),GOARM=7,) \
	go build $(BUILDOPTS) $(LDFLAGS) -o $@
