#!/bin/bash

# DocMind 构建脚本

set -e

echo "========================================"
echo "DocMind Build Script"
echo "========================================"

# 配置
APP_NAME="docmind-api"
BUILD_DIR="bin"
VERSION=${VERSION:-"1.0.0"}
BUILD_TIME=$(date +%Y-%m-%d\ %H:%M:%S)
GO_VERSION=$(go version | awk '{print $3}')
COMMIT_ID=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 创建构建目录
echo "Creating build directory..."
mkdir -p ${BUILD_DIR}

# 构建参数
LDFLAGS="-X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GoVersion=${GO_VERSION}' -X 'main.CommitID=${COMMIT_ID}'"

echo "Building ${APP_NAME}..."
echo "Version: ${VERSION}"
echo "Commit: ${COMMIT_ID}"
echo "Go Version: ${GO_VERSION}"
echo "========================================"

# 构建
go build -ldflags "${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME} cmd/server/main.go

echo "========================================"
echo "Build completed!"
echo "Output: ${BUILD_DIR}/${APP_NAME}"
echo "========================================"
