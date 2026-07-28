# 知识库管理 API

## POST 创建知识库

POST /knowledge-bases

> Body 请求参数

```json
{
    "name": "weknora",
    "description": "weknora description",
    "type": "document",
    "is_temporary": false,
    "chunking_config": {
        "chunk_size": 1000,
        "chunk_overlap": 200,
        "separators": [
            "."
        ],
        "enable_multimodal": true,
        "parser_engine_rules": [
            {
                "file_types": [
                    ".pdf",
                    ".docx"
                ],
                "engine": "builtin"
            }
        ],
        "enable_parent_child": false,
        "parent_chunk_size": 4096,
        "child_chunk_size": 384
    },
    "image_processing_config": {
        "model_id": "f2083ad7-63e3-486d-a610-e6c56e58d72e"
    },
    "embedding_model_id": "dff7bc94-7885-4dd1-bfd5-bd96e4df2fc3",
    "summary_model_id": "8aea788c-bb30-4898-809e-e40c14ffb48c",
    "vlm_config": {
        "enabled": true,
        "model_id": "f2083ad7-63e3-486d-a610-e6c56e58d72e"
    },
    "asr_config": {
        "enabled": false,
        "model_id": "",
        "language": ""
    },
    "storage_provider_config": {
        "provider": "local"
    },
    "storage_config": {
        "secret_id": "",
        "secret_key": "",
        "region": "",
        "bucket_name": "",
        "app_id": "",
        "path_prefix": ""
    },
    "extract_config": null,
    "faq_config": null,
    "question_generation_config": {
        "enabled": false,
        "question_count": 3
    },
    "vector_store_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|body|body|object| 是 |none|
|» name|body|string| 是 || description                   | string  | 否   | 知识库描述|
|» type|body|string| 否 || is_temporary                  | boolean | 否   | 是否为临时知识库（默认 `false`，临时库通常不在 UI 列表中显示）|
|» chunking_config|body|object| 否 || image_processing_config       | object  | 否   | 图片处理配置|
|» embedding_model_id|body|string| 否 || summary_model_id              | string  | 否   | 摘要模型 ID|
|» vlm_config|body|object| 否 || asr_config                    | object  | 否   | ASR（语音识别）配置|
|» storage_provider_config|body|object| 否 || storage_config                | object  | 否   | 旧版 COS 存储凭证（兼容字段，新集成留空即可）|
|» extract_config|body|object| 否 || faq_config                    | object  | 否   | FAQ 配置（仅 FAQ 类型知识库需要）|
|» question_generation_config|body|object| 否 || vector_store_id               | string  | 否   | 绑定的向量存储 ID。不传或为空字符串等同于 `null`（使用环境变量默认存储）。指定时必须是调用者所在空间拥有的向量存储 UUID；创建后不可修改。无效 UUID / 跨空间 / 未注册到引擎的 ID 会返回 `400`|

> 返回示例

> 200 Response

```json
{
    "data": {
        "id": "b5829e4a-3845-4624-a7fb-ea3b35e843b0",
        "name": "weknora",
        "description": "weknora description",
        "type": "document",
        "is_temporary": false,
        "tenant_id": 1,
        "chunking_config": {
            "chunk_size": 1000,
            "chunk_overlap": 200,
            "separators": [
                "."
            ],
            "enable_multimodal": true,
            "parser_engine_rules": [
                {
                    "file_types": [
                        ".pdf",
                        ".docx"
                    ],
                    "engine": "builtin"
                }
            ],
            "enable_parent_child": false,
            "parent_chunk_size": 4096,
            "child_chunk_size": 384
        },
        "image_processing_config": {
            "model_id": "f2083ad7-63e3-486d-a610-e6c56e58d72e"
        },
        "embedding_model_id": "dff7bc94-7885-4dd1-bfd5-bd96e4df2fc3",
        "summary_model_id": "8aea788c-bb30-4898-809e-e40c14ffb48c",
        "vlm_config": {
            "enabled": true,
            "model_id": "f2083ad7-63e3-486d-a610-e6c56e58d72e"
        },
        "asr_config": {
            "enabled": false,
            "model_id": "",
            "language": ""
        },
        "storage_provider_config": {
            "provider": "local"
        },
        "storage_config": {
            "secret_id": "",
            "secret_key": "",
            "region": "",
            "bucket_name": "",
            "app_id": "",
            "path_prefix": ""
        },
        "extract_config": null,
        "faq_config": null,
        "question_generation_config": {
            "enabled": false,
            "question_count": 3
        },
        "is_pinned": false,
        "pinned_at": null,
        "knowledge_count": 0,
        "chunk_count": 0,
        "processing_count": 0,
        "vector_store_id": "550e8400-e29b-41d4-a716-446655440000",
        "vector_store_name": "elasticsearch-hot",
        "vector_store_source": "user",
        "vector_store_engine_type": "elasticsearch",
        "vector_store_status": "available",
        "created_at": "2025-08-12T11:30:09.206238645+08:00",
        "updated_at": "2025-08-12T11:30:09.206238854+08:00",
        "deleted_at": null
    },
    "success": true
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|Inline|

### 返回数据结构

## GET 获取知识库列表

GET /knowledge-bases

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|None|

## GET 获取知识库详情

GET /knowledge-bases/{id}

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|id|path|string| 是 |知识库 ID|

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|None|

## PUT 更新知识库

PUT /knowledge-bases/{id}

> Body 请求参数

```json
{
    "name": "weknora new",
    "description": "weknora description new",
    "config": {
        "chunking_config": {
            "chunk_size": 1000,
            "chunk_overlap": 200,
            "separators": [
                "\n\n",
                "\n",
                "。",
                "！",
                "？",
                ";",
                "；"
            ],
            "enable_multimodal": true,
            "parser_engine_rules": [
                {
                    "file_types": [
                        ".md",
                        ".txt"
                    ],
                    "engine": "builtin"
                }
            ],
            "enable_parent_child": true,
            "parent_chunk_size": 4096,
            "child_chunk_size": 384
        },
        "image_processing_config": {
            "model_id": ""
        }
    }
}
```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|id|path|string| 是 |知识库 ID|
|body|body|object| 是 |none|
|» name|body|string| 是 || description | string | 否   | 知识库描述|

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|None|

## DELETE 删除知识库

DELETE /knowledge-bases/{id}

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|id|path|string| 是 |知识库 ID|

> 返回示例

> 200 Response

```json
{
    "message": "Knowledge base deleted successfully",
    "success": true
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|Inline|

### 返回数据结构

## PUT 置顶/取消置顶知识库

PUT /knowledge-bases/{id}/pin

> Body 请求参数

```
string

```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|id|path|string| 是 |知识库 ID|
|body|body|string| 是 |none|

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|None|

## POST 混合搜索

POST /knowledge-bases/{id}/hybrid-search

> Body 请求参数

```json
{
    "query_text": "如何使用知识库",
    "vector_threshold": 0.5,
    "match_count": 10
}
```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|id|path|string| 是 |知识库 ID|
|body|body|object| 是 |none|
|» query_text|body|string| 是 || vector_threshold         | number   | 否   | 向量相似度阈值（0-1）|
|» keyword_threshold|body|number| 否 || match_count              | integer  | 否   | 返回结果数量上限|
|» disable_keywords_match|body|boolean| 否 || disable_vector_match     | boolean  | 否   | 关闭向量召回|
|» knowledge_ids|body|string[]| 否 || tag_ids                  | string[] | 否   | 标签过滤（FAQ 类型常用于优先级过滤）|
|» only_recommended|body|boolean| 否 || knowledge_base_ids       | string[] | 否   | 跨知识库召回（需共享相同 embedding 模型），优先级高于路径中的 `:id`|

> 返回示例

> 200 Response

```json
{
    "data": [
        {
            "id": "chunk-00000001",
            "content": "知识库是用于存储和检索知识的系统...",
            "knowledge_id": "knowledge-00000001",
            "chunk_index": 0,
            "knowledge_title": "知识库使用指南",
            "start_at": 0,
            "end_at": 500,
            "seq": 1,
            "score": 0.95,
            "chunk_type": "text",
            "image_info": "",
            "metadata": {},
            "knowledge_filename": "guide.pdf",
            "knowledge_source": "file"
        }
    ],
    "success": true
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|成功|Inline|

### 返回数据结构



