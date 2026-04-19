#!/bin/bash

app_name="main.go"
output_name="main"

# Проверяем наличие go.mod
if [ ! -f "go.mod" ]; then
    echo "Инициализация Go модуля..."
    go mod init gobank
fi

# Проверяем версию Go в go.mod и исправляем если нужно
if grep -q "go 1.25.0" go.mod; then
    echo "Исправляем версию Go..."
    sed -i 's/go 1.25.0/go 1.22/' go.mod
fi

# Скачиваем зависимости
echo "Скачивание зависимостей..."
go mod tidy

echo "Building $app_name..."

# Компилируем
go build -o "$output_name" "$app_name"

if [ $? -eq 0 ]; then
    echo "Build Complete! Запусти ./$output_name"
else
    echo "Build Failed!"
    exit 1
fi