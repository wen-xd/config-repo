func (h helpDocRepo) CronHelpDocTask(ctx context.Context) error {
	ctx = data.GetTxCtx(ctx, h.data.GetDb(ctx))
	if err := syncData(ctx, h.es); err != nil {
		log.Printf("全量同步失败: %v", err)
		return err
	}
	return nil
}
func syncData(ctx context.Context, eClient *esUtil.EsClient) error {
	newIndex := fmt.Sprintf("%s_%s", HelpDocIndex, time.Now().Format("20060102150405"))
	fmt.Printf("new index name : %s\n", newIndex)
	// 1. 创建新索引
	err := eClient.CreateIndexNx(Mapping, newIndex)
	if err != nil {
		return err
	}

	// 2. 同步数据到新索引
	if err := syncDataToNew(ctx, eClient, newIndex); err != nil {
		// 同步失败，删除新索引
		eClient.DeleteIndex(newIndex)
		return err
	}

	// 3. 更新别名
	m, err := eClient.GetIndexWithAlias("help_docs")
	if err != nil {
		return err
	}
	fmt.Printf("map len ：%d\n", len(m))
	var old string
	for oldIndex := range m {
		if err := eClient.SwitchAlias(ctx, "help_docs", oldIndex, newIndex); err != nil {
			eClient.DeleteIndex(newIndex)
			return fmt.Errorf("更新别名失败: %v", err)
		}
		old = oldIndex
		log.Printf("oldIndex: %s", oldIndex)
		break
	}

	// 4. 清理旧索引
	if err := eClient.DeleteIndex(old); err != nil {
		return fmt.Errorf("清理旧索引失败: %v", err)
	}

	return nil
}

// 批量写入 ES（支持插入/更新）
func syncDataToNew(ctx context.Context, eClient *esUtil.EsClient, newIndex string) error {
	db := data.GetDbFromContext(ctx)
	var docs []entity.HelpDocModel
	if err := data.And().BuildWhere(db).Unscoped().Find(&docs).Error; err != nil {
		return errors.New("update_at err")
	}
	bulk := make([]interface{}, 0, len(docs)*2)
	// Process each document
	for _, doc := range docs {
		if !doc.DeletedAt.Time.IsZero() {
			if err := data.C("id", doc.Id).BuildWhere(db).Unscoped().Delete(&entity.HelpDocModel{}); err != nil {
				log.Printf("Failed to delete help doc from DB: %v", err)
				continue
			}
		}
		esDoc := &HelpDocES{
			ID:          doc.Id,
			ParentId:    doc.ParentId,
			SortNumber:  doc.SortNumber,
			Title:       doc.Title,
			Status:      doc.Status,
			Content:     regexp.MustCompile(`<img[^>]*>`).ReplaceAllString(doc.Content, ""),
			Description: doc.Description,
			Keywords:    doc.Keywords,
		}
		bulk = append(bulk, map[string]interface{}{
			"index": map[string]interface{}{
				"_index": newIndex,
				"_id":    strconv.FormatInt(doc.Id, 10),
			},
		})
		bulk = append(bulk, esDoc)
	}
	// 执行批量操作
	if len(bulk) > 0 {
		if err := eClient.Bulk(newIndex, bulk); err != nil {
			return fmt.Errorf("批量索引失败: %v", err)
		}
	}

	return nil
}

type HelpDocES struct {
	ID          int64  `json:"id"`
	ParentId    int64  `json:"parent_id"`
	SortNumber  int64  `json:"sort_number"`
	Title       string `json:"title"`
	Status      int32  `json:"status"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Keywords    string `json:"keywords"`
	//Paths       []*entity.PathNode `json:"paths"`
}

// ****************************  Abandoned  ***************************

const (
	HelpDocIndex = "help_docs"
	Mapping      = `{
		"settings": {
			"analysis": {
			  "analyzer": {
				"ik_max_word_analyzer": { 
				  "type": "custom",
				  "tokenizer": "ik_max_word"
				},
				"ik_smart_analyzer": { 
				  "type": "custom",
				  "tokenizer": "ik_smart"
				}
			  }
			}
		  },
		"mappings": {
			"properties": {
				"id": { "type": "long" },
				"parent_id": { "type": "long" },
				"sort_number": { "type": "long" },
				"title": { "type": "text", "analyzer": "ik_max_word_analyzer","search_analyzer": "ik_smart_analyzer" },
				"status": { "type": "integer" },
				"content": { "type": "text", "analyzer": "ik_max_word_analyzer","search_analyzer": "ik_smart_analyzer" },
				"description": { "type": "text", "analyzer": "ik_max_word_analyzer","search_analyzer": "ik_smart_analyzer" },
				"keywords": { "type": "text", "analyzer": "ik_max_word_analyzer","search_analyzer": "ik_smart_analyzer" },
				"paths": { "type": "text", "analyzer": "ik_max_word_analyzer","search_analyzer": "ik_smart_analyzer" }
			}
		}
	}`
)
