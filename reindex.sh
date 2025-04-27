#!/usr/bin/env bash

# 配置
ES_HOST="http://10.0.1.102:9200"  # Elasticsearch地址
SOURCE_INDEX="help_docs_20250421004000"    # 原索引名
TARGET_INDEX="rehelp"              # 新索引名
ALIAS_NAME="help_docs"               # 别名

# 1. 检查原索引是否存在别名
echo "检查原索引的别名..."
ALIAS_EXISTS=$(curl -s -o /dev/null -w "%{http_code}" "$ES_HOST/$SOURCE_INDEX/_alias/$ALIAS_NAME")

if [ "$ALIAS_EXISTS" != "200" ]; then
  echo "警告: 原索引 $SOURCE_INDEX 没有别名 $ALIAS_NAME"
else
  echo "原索引 $SOURCE_INDEX 存在别名 $ALIAS_NAME"
fi

# 2. 执行 Reindex
echo "开始 Reindex 从 $SOURCE_INDEX 到 $TARGET_INDEX..."
REINDEX_RESPONSE=$(curl -s -X POST "$ES_HOST/_reindex" -H "Content-Type: application/json" -d '{
  "source": {
    "index": "'"$SOURCE_INDEX"'"
  },
  "dest": {
    "index": "'"$TARGET_INDEX"'"
  }
}')

# 检查 Reindex 是否成功
if echo "$REINDEX_RESPONSE" | jq -e '.error' > /dev/null; then
  echo "Reindex 失败: $REINDEX_RESPONSE"
  exit 1
else
  echo "Reindex 成功完成！"
fi

# 3. 迁移别名（如果存在）
if [ "$ALIAS_EXISTS" = "200" ]; then
  echo "迁移别名 $ALIAS_NAME 到新索引..."
  curl -X POST "$ES_HOST/_aliases" -H "Content-Type: application/json" -d '{
    "actions": [
      { "remove": { "index": "'"$SOURCE_INDEX"'", "alias": "'"$ALIAS_NAME"'" }},
      { "add": { "index": "'"$TARGET_INDEX"'", "alias": "'"$ALIAS_NAME"'" }}
    ]
  }'
  echo "别名迁移完成！"
fi

# 4. 删除所有 help* 索引（排除新索引）
echo "准备删除旧索引..."
OLD_INDICES=$(curl -s "$ES_HOST/_cat/indices/help*?format=json" | jq -r '.[].index' | grep -v "^$TARGET_INDEX$")

if [ -z "$OLD_INDICES" ]; then
  echo "没有找到可删除的旧索引"
else
  echo "即将删除以下索引:"
  echo "$OLD_INDICES"
  read -p "确认删除？(y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    for INDEX in $OLD_INDICES; do
      echo "删除索引: $INDEX"
      curl -X DELETE "$ES_HOST/$INDEX"
    done
    echo "旧索引已全部删除！"
  else
    echo "已取消删除操作"
  fi
fi