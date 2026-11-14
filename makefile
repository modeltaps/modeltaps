NAME=modeltaps
DISTDIR=dist
WEBDIR=web
VERSION=$(shell git describe --tags || echo "dev")
GOBUILD=go build -ldflags "-s -w -X 'github.com/modeltaps/modeltaps/common/config.Version=$(VERSION)'"

all: modeltaps

.PHONY: dev clean

web: $(WEBDIR)/build

$(WEBDIR)/build:
	cd $(WEBDIR) && pnpm install --frozen-lockfile --filter modeltaps-web && VITE_APP_VERSION=$(VERSION) pnpm run build

modeltaps: web
	$(GOBUILD) -o $(DISTDIR)/$(NAME)

# Development mode: backend + Vite, both reloading on change (see scripts/dev.sh)
dev:
	./scripts/dev.sh

clean:
	rm -rf $(DISTDIR) && rm -rf $(WEBDIR)/build
