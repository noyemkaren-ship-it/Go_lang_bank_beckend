#!/bin/bash

app_name="main.go"


if [ ! -f "go.mod" ]; then
    echo "Инициализация Go модуля..."
    go mod init my_app
fi

echo "Building $app_name..."

go build "$app_name"

if [ $? -eq 0 ]; then
    echo "Build Complete"
else
    echo "Build Failed!"
fi