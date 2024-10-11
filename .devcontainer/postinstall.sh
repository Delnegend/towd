#!/bin/sh

npm i -g pnpm
pnpm config set store-dir /home/vscode/.pnpm-store
go install go.abhg.dev/requiredfield/cmd/requiredfield@latest
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
