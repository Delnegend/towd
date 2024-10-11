#!/bin/sh

npm i -g pnpm
pnpm config set store-dir /home/vscode/.pnpm-store
go install go.abhg.dev/requiredfield/cmd/requiredfield@latest
