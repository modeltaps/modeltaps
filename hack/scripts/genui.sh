#!/usr/bin/env bash

version=${1}

if [ -d "web" ]; then
  pushd web || exit
  cp package.json package.json.example
  # cat package.json  | jq '.version="'${version}'"' > package.json.new
  jq '.version="'"${version}"'"' package.json > package.json.new && mv package.json.new package.json
  pnpm install --frozen-lockfile --filter modeltaps-web
  VITE_APP_VERSION=$version pnpm build
  mv package.json.example package.json
  popd || exit
fi
