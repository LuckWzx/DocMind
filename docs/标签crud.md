---
title: 默认模块
language_tabs:
  - shell: Shell
  - http: HTTP
  - javascript: JavaScript
  - ruby: Ruby
  - python: Python
  - php: PHP
  - java: Java
  - go: Go
toc_footers: []
includes: []
search: true
code_clipboard: true
highlight_theme: darkula
headingLevel: 2
generator: "@tarslib/widdershins v4.0.30"

---

# 默认模块

WeKnora 知识库管理系统 API 文档

Base URLs:

# Authentication

# 标签管理 API

## GET 获取知识库标签列表

GET /knowledge-bases/{id}/tags

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|id|path|string| 是 |none|

> 返回示例

> 200 Response

```json
{
    "data": {
        "total": 2,
        "page": 1,
        "page_size": 10,
        "data": [
            {
                "id": "tag-00000001",
                "tenant_id": 1,
                "knowledge_base_id": "kb-00000001",
                "name": "技术文档",
                "color": "#1890ff",
                "sort_order": 1,
                "created_at": "2025-08-12T10:00:00+08:00",
                "updated_at": "2025-08-12T10:00:00+08:00",
                "knowledge_count": 5,
                "chunk_count": 120
            },
            {
                "id": "tag-00000002",
                "tenant_id": 1,
                "knowledge_base_id": "kb-00000001",
                "name": "常见问题",
                "color": "#52c41a",
                "sort_order": 2,
                "created_at": "2025-08-12T10:00:00+08:00",
                "updated_at": "2025-08-12T10:00:00+08:00",
                "knowledge_count": 3,
                "chunk_count": 45
            }
        ]
    },
    "success": true
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|Inline|

### 返回数据结构

## POST 创建标签

POST /knowledge-bases/{id}/tags

> Body 请求参数

```json
{
    "name": "产品手册",
    "color": "#faad14",
    "sort_order": 3
}
```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|id|path|string| 是 |知识库 ID|
|body|body|object| 是 |none|
|» name|body|string| 是 || color      | string | 否   | 标签颜色（CSS 颜色字符串）|

> 返回示例

> 200 Response

```json
{
    "data": {
        "id": "tag-00000003",
        "tenant_id": 1,
        "knowledge_base_id": "kb-00000001",
        "name": "产品手册",
        "color": "#faad14",
        "sort_order": 3,
        "created_at": "2025-08-12T11:00:00+08:00",
        "updated_at": "2025-08-12T11:00:00+08:00"
    },
    "success": true
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|Inline|

### 返回数据结构

## PUT 更新标签

PUT /knowledge-bases/{id}/tags/{tag_id}

> Body 请求参数

```json
{
    "name": "产品手册更新",
    "color": "#ff4d4f"
}
```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|------|path|------| 是 |-----------|
|id|path|string| 是 |知识库 ID|
|tag_id|path|string| 是 |标签 ID|
|body|body|object| 是 |none|

> 返回示例

> 200 Response

```json
{
    "data": {
        "id": "tag-00000003",
        "tenant_id": 1,
        "knowledge_base_id": "kb-00000001",
        "name": "产品手册更新",
        "color": "#ff4d4f",
        "sort_order": 3,
        "created_at": "2025-08-12T11:00:00+08:00",
        "updated_at": "2025-08-12T11:30:00+08:00"
    },
    "success": true
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|Inline|

### 返回数据结构

## DELETE 删除标签

DELETE /knowledge-bases/{id}/tags/{tag_id}

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|------|path|------| 是 |--------|
|id|path|string| 是 |知识库 ID|
|tag_id|path|string| 是 |标签 ID|
|-----|query|-------| 否 |---------------------------------------------|
|force|query|boolean| 否 |设置为 `true` 时强制删除（即使标签被引用）|

> 返回示例

> 200 Response

```json
{
    "success": true
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|Inline|

### 返回数据结构

# 数据模型

