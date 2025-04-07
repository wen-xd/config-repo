package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrRequestFailed    = errors.New("request failed")
)

// ClientConfig 客户端配置
type ClientConfig struct {
	Addresses   []string // ES节点地址列表
	Username    string   // 用户名（Basic Auth）
	Password    string   // 密码（Basic Auth）
	APIKey      string   // API密钥
	CACertPath  string   // CA证书路径
	EnableDebug bool     // 启用调试模式
	MaxRetries  int      // 最大重试次数
	Timeout     int      // 请求超时（秒）
}

// EsClient Elasticsearch 客户端
type EsClient struct {
	es *elasticsearch.Client
}

// NewEsClient 创建新的ES客户端
func NewEsClient(cfg ClientConfig) (*EsClient, error) {
	// 1. 初始化基础配置
	esCfg := elasticsearch.Config{
		Addresses:  cfg.Addresses,
		MaxRetries: cfg.MaxRetries,
	}

	// 2. 配置TLS
	if cfg.CACertPath != "" {
		caCert, err := ioutil.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		esCfg.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		}
	}

	// 3. 配置认证
	if cfg.Username != "" && cfg.Password != "" {
		esCfg.Username = cfg.Username
		esCfg.Password = cfg.Password
	} else if cfg.APIKey != "" {
		esCfg.APIKey = cfg.APIKey
	}

	// 4. 配置超时
	if cfg.Timeout > 0 {
		esCfg.Transport = &http.Transport{
			ResponseHeaderTimeout: time.Duration(cfg.Timeout) * time.Second,
		}
	}

	// 5. 初始化客户端
	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ES client: %w", err)
	}

	return &EsClient{es: client}, nil
}

// Index 索引文档
func (c *EsClient) Index(ctx context.Context, index, documentID string, body io.Reader) (map[string]interface{}, error) {
	resp, err := esapi.IndexRequest{
		Index:      index,
		DocumentID: documentID,
		Body:       body,
		Refresh:    "true",
	}.Do(ctx, c.es)
	return c.handleResponse(resp, err)
}
func (c *EsClient) Delete(ctx context.Context, index, documentID string) (map[string]interface{}, error) {
	resp, err := esapi.DeleteRequest{
		Index:      index,
		DocumentID: documentID,
		Refresh:    "true",
	}.Do(ctx, c.es)
	return c.handleResponse(resp, err)
}

// Search 执行搜索
func (c *EsClient) Search(ctx context.Context, index string, query io.Reader) (map[string]interface{}, error) {
	resp, err := esapi.SearchRequest{
		Index: []string{index},
		Body:  query,
	}.Do(ctx, c.es)

	return c.handleResponse(resp, err)
}

// Get 获取文档
func (c *EsClient) Get(ctx context.Context, index, documentID string) (map[string]interface{}, error) {
	resp, err := esapi.GetRequest{
		Index:      index,
		DocumentID: documentID,
	}.Do(ctx, c.es)

	return c.handleResponse(resp, err)
}
func (c *EsClient) ReindexData(ctx context.Context, sourceIndex, destIndex string) error {
	query := `{
		"source": {
			"index": "%s"
		},
		"dest": {
			"index": "%s",
			"op_type": "create"
		},
		"conflicts": "proceed"
	}`

	req := esapi.ReindexRequest{
		Body: strings.NewReader(fmt.Sprintf(query, sourceIndex, destIndex)),
	}
	res, err := req.Do(ctx, c.es)
	_, err = c.handleResponse(res, err)
	if err != nil {
		return err
	}
	// 监控reindex任务状态
	taskID := res.Header.Get("X-Opaque-Id")
	return c.monitorTask(ctx, taskID)
}

// DeleteIndex 删除索引
func (c *EsClient) DeleteIndex(index string) error {
	res, err := c.es.Indices.Delete([]string{index})
	if err != nil {
		return fmt.Errorf("failed to delete index: %w", err)
	}
	_, err = c.handleResponse(res, err)
	return err
}
func (c *EsClient) SwitchAlias(ctx context.Context, aliasName, oldIndex, newIndex string) error {
	actions := []map[string]interface{}{
		{"remove": map[string]string{"index": oldIndex, "alias": aliasName}},
		{"add": map[string]string{"index": newIndex, "alias": aliasName}},
	}

	body := map[string]interface{}{"actions": actions}
	jsonBody, _ := json.Marshal(body)

	req := esapi.IndicesUpdateAliasesRequest{
		Body: bytes.NewReader(jsonBody),
	}
	res, err := req.Do(ctx, c.es)
	_, err = c.handleResponse(res, err)
	return err
}

// 处理响应和错误
func (c *EsClient) handleResponse(resp *esapi.Response, err error) (map[string]interface{}, error) {
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, parseErrorResponse(resp)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
func (c *EsClient) monitorTask(ctx context.Context, taskID string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			req := esapi.TasksGetRequest{
				TaskID: taskID,
			}

			res, err := req.Do(ctx, c.es)
			_, err = c.handleResponse(res, err)
			if err != nil {
				return fmt.Errorf("get task status error: %w", err)
			}
			var taskResp struct {
				Completed bool `json:"completed"`
				Error     struct {
					Reason string `json:"reason"`
				} `json:"error"`
			}

			if err := json.NewDecoder(res.Body).Decode(&taskResp); err != nil {
				return fmt.Errorf("decode task response error: %w", err)
			}

			if taskResp.Error.Reason != "" {
				return fmt.Errorf("task failed: %s", taskResp.Error.Reason)
			}

			if taskResp.Completed {
				return nil
			}
		}
	}
}
func (c *EsClient) Bulk(index string, data []interface{}) error {
	// 构建批量请求的body
	var buf bytes.Buffer
	for _, doc := range data {
		// 将每个文档转换为JSON
		docBytes, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("marshal document failed: %v", err)
		}

		// 写入到buffer
		buf.Write(docBytes)
		// 添加换行符
		buf.Write([]byte("\n"))
	}

	// 执行批量操作
	res, err := c.es.Bulk(
		bytes.NewReader(buf.Bytes()),
		c.es.Bulk.WithIndex(index),
		c.es.Bulk.WithRefresh("true"),
	)
	_, err = c.handleResponse(res, err)
	return err
}
func (c *EsClient) CreateIndexNx(mapping, index string) error {
	res, err := c.es.Indices.Exists([]string{index})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		res, err = c.es.Indices.Create(
			index,
			c.es.Indices.Create.WithBody(strings.NewReader(mapping)),
		)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		if res.IsError() {
			return fmt.Errorf("error creating index: %s", res.String())
		}
	}

	return nil
}

// GetAlias 获取别名指向的索引列表
func (c *EsClient) GetIndexWithAlias(alias string) (map[string]bool, error) {
	// 执行获取别名的请求
	res, err := c.es.Indices.GetAlias(
		c.es.Indices.GetAlias.WithName(alias),
	)
	result, err := c.handleResponse(res, err)
	if err != nil {
		return nil, err
	}
	// 提取索引名称
	indices := make(map[string]bool)
	for indexName := range result {
		// 跳过元数据字段
		if strings.HasPrefix(indexName, ".") {
			continue
		}
		indices[indexName] = true
	}

	return indices, nil
}

// 解析错误响应
func parseErrorResponse(resp *esapi.Response) error {
	switch resp.StatusCode {
	case http.StatusNotFound:
		return ErrDocumentNotFound
	case http.StatusBadRequest, http.StatusInternalServerError:
		var errRes map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errRes); err != nil {
			return fmt.Errorf("%w: %s", ErrRequestFailed, resp.Status())
		}
		if reason, ok := errRes["error"].(map[string]interface{})["reason"]; ok {
			return fmt.Errorf("%w: %s", ErrRequestFailed, reason)
		}
		return fmt.Errorf("%w: %s", ErrRequestFailed, resp.Status())
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// 日志传输层（调试模式）
type loggingTransport struct {
	transport http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("Request: %s %s\n", req.Method, req.URL)
	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Response: %s\n", resp.Status)
	return resp, nil
}
