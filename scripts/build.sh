#!/bin/bash

# CloudQue 构建脚本

set -e

echo "========================================"
echo "CloudQue Build Script"
echo "========================================"

# 配置
APP_NAME="cloudque-api"
BUILD_DIR="bin"
VERSION=${VERSION:-"1.0.0"}
BUILD_TIME=$(date +%Y-%m-%d\ %H:%M:%S)
GO_VERSION=$(go version | awk '{print $3}')

# 创建构建目录
echo "Creating build directory..."
mkdir -p ${BUILD_DIR}

# 构建参数
LDFLAGS="-X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GoVersion=${GO_VERSION}'"

echo "Building ${APP_NAME}..."
echo "Version: ${VERSION}"
echo "Go Version: ${GO_VERSION}"
echo "========================================"

# 构建
go build -ldflags "${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME} cmd/server/main.go

echo "========================================"
echo "Build completed!"
echo "Output: ${BUILD_DIR}/${APP_NAME}"
echo "========================================"
