NAME=modeltaps
DISTDIR=dist
WEBDIR=web
VERSION=$(shell git describe --tags || echo "dev")
GOBUILD=go build -ldflags "-s -w -X 'github.com/modeltaps/modeltaps/common/config.Version=$(VERSION)'"

all: modeltaps

web: $(WEBDIR)/build

$(WEBDIR)/build:
	cd $(WEBDIR) && pnpm install --frozen-lockfile --filter modeltaps-web && VITE_APP_VERSION=$(VERSION) pnpm run build

modeltaps: web
	$(GOBUILD) -o $(DISTDIR)/$(NAME)

clean:
	rm -rf $(DISTDIR) && rm -rf $(WEBDIR)/build
