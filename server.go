package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HelpDocModal 文档模型
type HelpDocModal struct {
	Id          int64  `json:"id"`
	ParentId    int64  `json:"parent_id"`
	SortNumber  int64  `json:"sort_number"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Keywords    string `json:"keywords"`
	Paths       string `json:"paths"`
}

// Anchor 锚点数据结构
type Anchor struct {
	ID   string
	Text string
}

// PathItem 路径项数据结构
type PathItem struct {
	Id    int64  `json:"id"`
	Title string `json:"title"`
}

// PageData 页面数据结构
type PageData struct {
	Title   string
	Content template.HTML
	Anchors []Anchor
	Paths   []PathItem
}

// 模拟ES搜索接口
func searchDocuments(keyword string) ([]*HelpDocModal, error) {
	// 这里是模拟数据，实际应用中应该调用ES搜索
	return []*HelpDocModal{
		{
			Id:          1,
			Title:       "刷新和预热资源",
			Content:     "<h2>刷新和预热资源</h2><p>当您的源站内容更新后，为了保证用户访问到的是最新内容，您可以使用刷新功能来强制CDN节点更新缓存。</p>",
			Description: "刷新和预热CDN资源的指南",
		},
		{
			Id:          2,
			Title:       "域名管理",
			Content:     "<h2>域名管理</h2><p>您可以通过域名管理功能添加、修改和删除CDN加速域名。</p>",
			Description: "CDN域名管理指南",
		},
	}, nil
}

// 解析HTML内容中的锚点，并返回修改后的HTML内容和锚点列表
func parseAnchors(content string) (string, []Anchor) {
	// 创建一个reader
	reader := strings.NewReader(content)
	// 使用goquery解析HTML
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		log.Printf("解析HTML失败: %v", err)
		return content, nil
	}

	// 查找所有的标题元素
	var anchors []Anchor
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		// 生成ID (使用标题序号作为前缀，确保唯一性和规范性)
		id := fmt.Sprintf("heading-%d", i+1)
		// 如果需要，可以添加基于文本的后缀，但要确保只包含安全字符
		suffix := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(text, "-")
		if suffix != "" {
			id = id + "-" + strings.Trim(strings.ToLower(suffix), "-")
		}
		// 添加到锚点列表
		anchors = append(anchors, Anchor{ID: id, Text: text})
		// 为原始HTML元素添加ID属性
		s.SetAttr("id", id)
	})

	// 将修改后的HTML转换回字符串
	modifiedHTML, err := doc.Html()
	if err != nil {
		log.Printf("转换HTML失败: %v", err)
		return content, anchors
	}

	return modifiedHTML, anchors
}

type HelpDocModel struct {
	Id          int64         `query:"id" json:"id" label:"id" validate:"required"`
	ParentId    int64         `query:"parent_id" json:"parent_id" label:"父目录id" validate:"required"`
	Title       string        `query:"title" json:"title" label:"标题" validate:"required"`
	SortNumber  int64         `query:"sort_number" json:"sort_number" label:"层级排序键"`
	Status      int32         `query:"status" json:"status" label:"发布状态：0（未发布）或 1（已发布）" validate:"required"`
	Content     template.HTML `query:"content" json:"content" label:"页面内容" validate:"required"`
	Description string        `query:"description" json:"description" label:"页面简要描述" validate:"required"`
	Keywords    string        `query:"keywords" json:"keywords" label:"关键词" validate:"required"`
	Paths       string        `query:"paths" json:"paths" label:"当前的层级关系" validate:"required"`
	CreateAt    string        `query:"create_at" json:"create_at" label:"create_at"`
	UpdateAt    string        `query:"update_at" json:"update_at" label:"update_at"`
	UpId        int64         `query:"UpId" json:"UpId" label:"上一篇ID" validate:"required"`
	DownId      int64         `query:"DownId" json:"DownId" label:"下一篇ID" validate:"required"`
	UpTitle     string        `query:"UpTitle" json:"UpTitle" label:"上一篇标题" validate:"required"`
	DownTitle   string        `query:"DownTitle" json:"DownTitle" label:"下一篇标题" validate:"required"`
}

// 获取目录列表的API处理函数
func getDirectoryList(w http.ResponseWriter, r *http.Request) {
	// 模拟从数据库获取目录数据
	directoryList := []PathItem{
		{Id: 1, Title: "用户指南"},
		{Id: 2, Title: "产品介绍"},
		{Id: 3, Title: "快速入门"},
		{Id: 4, Title: "常见问题"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(directoryList)
}

// func main() {
// 	hdm := HelpDocModal{
// 		Id:          1,
// 		Title:       "刷新和预热资源",
// 		Content:     `<h2>用户端帮助文档</h2><h3>关于AIWCDN</h3><h3>产品概述</h3><h4>产品简介</h4><p>AIWCDN不仅仅是一个内容分发网络，还是一种综合的网络安全解决方案，提供了多层次、多维度的防护措施，帮助企业抵御各种常见的网络攻击，提升访问速度的同时，<br>保障用户数据和网站服务的安全性。</p><p><strong>为什么选择AIWCDN</strong><br>全球节点覆盖：提供多个海外节点，覆盖更广泛的区域，确保全球用户都能享受低延迟、高速度的访问体验。<br><br>智能加速：通过智能路由优化技术，动态选择最优路径，大幅提升内容加载速度。<br><br>防CC攻击：支持多种防CC模式，包括请5秒盾、验证码验证及滑动验证等，灵活适配不同业务场景，全面满足多样化的防CC攻击需求。<br><br>实时监控与报表：提供更详细的实时监控数据与可视化报表，帮助您更好地了解资源使用情况与性能表现。<br><br>API接口：提供多项API功能，便于开发者更灵活地集成与管理CDN服务。</p><p><strong>产品架构图</strong><img src="./images/jiagou.jpg" alt="jiagou.jpg" data-href="" style=""/></p><p><strong>智能解析系统</strong><br>智能解析系统会实时监测所有节点和链路的实时负载以及健康状况，并将结果反馈给智能调度系统，调度系统根据用户IP地址分配用户一个最佳接入节点。从而显著降低用户访问延迟、提升内容加载速度</p><p><strong>节点系统</strong><br>当用户发起请求时，CDN系统会首先检查边缘节点是否缓存了所需内容。如果缓存命中，边缘节点（L1）直接响应请求；如果未命中，则向上一级的节点（L2节点）请求内容。<br>L2节点同样会检查缓存是否命中，如果未命中，则从源站获取内容，并逐级向下分发，最终将内容传递给用户。<br>节点系统是CDN系统的主要部分，其作用包括以下几方面：<br>1、边缘节点分布在全球各地的网络边缘，能够就近响应用户请求，大幅减少数据传输距离，降低访问延迟。<br>2、将静态资源（如图片、视频、CSS、JavaScript 文件等）缓存到边缘节点，减少回源请求，提升内容加载速度。<br>3、通过边缘节点直接响应用户请求，减少对源站的访问压力，提高源站的稳定性和承载能力。<br>4、对于动态内容，边缘节点可以通过智能路由和协议优化技术，加速数据传输，提升用户体验。<br>5、边缘节点集成安全防护功能（如 DDoS 防护、CC 攻击防御等），在靠近用户的位置拦截恶意流量，保护源站安全。<br>6、通过多个边缘节点分担流量，实现负载均衡，避免单点过载，确保服务的高可用性。</p><p>分级部署的节点系统可就近服务用户，通过缓存、加速、安全防护和负载均衡等功能，提升内容分发的效率、稳定性和安全性，同时减轻源站压力，为用户提供更快速、可靠的访问体验。</p><h4>产品计费</h4><p>AIWcdn 提供套餐包年包月包季的预付费模式</p><p><strong>计费选项</strong><br>套餐：免费套餐、常规版、热门版、大带宽、防御版套餐，具体套餐内容，请参考套餐详情页面<br>升级包：在基础套餐的基础上叠加的带宽、流量和功能权限等服务,可根据需求购买升级包</p><p><strong>计费周期</strong><br>自购买之日起，根据所选择的购买时长（月/季/年）计算；购买时长支持选择1个月、3个月、1年。</p><p><strong>到期说明</strong><br>当购买的套餐到期后，CDN服务自动停止，CDN服务将不可用<br>距离到期前7天，您会收到邮件和站内信等通知，提醒您服务即将到期并及时续费</p><h3>配置指南</h3><h4>域名管理</h4><h5>添加域名</h5><p>新增单个域名</p><ol><li>登录控制台；</li><li>点击左侧菜单域名管理，进入域名管理列表；</li><li>点击新增，选择添加单个域名；</li><li>填写加速域名基础信息；</li></ol><p><img src="./images/domain-2.jpg" alt="domain-2.jpg" data-href="" style=""/></p><table style="width: auto;"><tbody><tr><th colSpan="1" rowSpan="1" width="auto">配置项</th><th colSpan="1" rowSpan="1" width="auto">说明</th></tr><tr><td colSpan="1" rowSpan="1" width="auto">域名</td><td colSpan="1" rowSpan="1" width="auto">域名长度不长于81个字符<br>支持英文字母26个字母不区分大小写、数字（0-9）<br> “-”,“-” 不能连续出现，也不能放在开头和结尾 <br> &nbsp;支持添加泛域名作为加速域名,如您在CDN添加泛域名*.test.com作为加速域名，并将*.test.com解析至CDN生成的CNAME域名后，那么您所有*.test.com的次级域名（如a.test.com）都将默认支持CDN加速。泛域名（*.test.com）的三级域名（如b.a.test.com）则不会被CDN加速。</td></tr><tr><td colSpan="1" rowSpan="1" width="auto">套餐</td><td colSpan="1" rowSpan="1" width="auto">选择已购买的套餐</td></tr><tr><td colSpan="1" rowSpan="1" width="auto">回源协议</td><td colSpan="1" rowSpan="1" width="auto"><strong>HTTP</strong>：回源请求采用 HTTP协议回源，如未自定义源站端口的情况下，默认使用80端口回源。<br><strong>HTTPS</strong>：回源请求采用 HTTPS 请求，如未自定义源站端口的情况下，默认使用443端口回源。</td></tr><tr><td colSpan="1" rowSpan="1" width="auto">源站地址</td><td colSpan="1" rowSpan="1" width="auto"><strong>地址</strong> 支持输入域名或IP地址 <br> &nbsp;<strong>端口</strong> 支持用户指定回源使用的访问端口，HTTP协议默认80，HTTPS协议默认443，如无修改可使用默认值，CDN 将根据回源协议使用默认端口回源。<br><strong>权重</strong> 当配置多个源站时，可以为每个源站配置权重，CDN回源时将按照权重轮询回源。权重取值1-100，数字越大回源次数越多</td></tr></tbody></table><p><br></p>`,
// 		Description: "刷新和预热CDN资源的指南",
// 		Paths:       `[{"id":28,"title":"dsds"},{"id":100,"title":"f"}]`,
// 	}
// 	// 创建模板函数映射
// 	funcMap := template.FuncMap{
// 		"safeHTML": func(s string) template.HTML {
// 			return template.HTML(s)
// 		},
// 		"split": strings.Split,
// 	}

// 	// 解析模板并添加函数映射
// 	tmpl, err := template.New("help_doc.tmpl").Funcs(funcMap).ParseFiles("help_doc.tmpl")
// 	if err != nil {
// 		log.Fatalf("解析模板失败: %v", err)
// 	}

// 	// 处理文档页面请求
// 	http.HandleFunc("/doc/", func(w http.ResponseWriter, r *http.Request) {
// 		// 从URL中提取文档ID
// 		pathParts := strings.Split(r.URL.Path, "/")
// 		if len(pathParts) < 3 {
// 			http.Error(w, "无效的文档ID", http.StatusBadRequest)
// 			return
// 		}

// 		// 这里应该根据ID从数据库或ES中获取文档
// 		// 这里使用模拟数据 - 直接使用main函数中定义的hdm变量

// 		// 解析HTML内容中的锚点，并获取修改后的HTML内容
// 		modifiedHTML, anchors := parseAnchors(hdm.Content)

// 		// 解析paths字段为PathItem数组
// 		var pathItems []PathItem
// 		err = json.Unmarshal([]byte(hdm.Paths), &pathItems)
// 		if err != nil {
// 			log.Printf("解析Paths字段失败: %v", err)
// 			// 如果解析失败，使用空数组
// 			pathItems = []PathItem{}
// 		}

// 		// 准备页面数据
// 		data := PageData{
// 			Title:   "刷新和预热资源",
// 			Content: template.HTML(modifiedHTML), // 使用修改后的HTML内容
// 			Anchors: anchors,
// 			Paths:   pathItems,
// 		}

// 		// 渲染模板
// 		err := tmpl.Execute(w, data)
// 		if err != nil {
// 			log.Printf("渲染模板失败: %v", err)
// 			http.Error(w, "内部服务器错误", http.StatusInternalServerError)
// 		}
// 	})

// 	// 注册目录API路由
// 	http.HandleFunc("/api/directory", getDirectoryList)

// 	// 处理搜索API请求
// 	http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
// 		// 获取搜索关键词
// 		keyword := r.URL.Query().Get("keyword")
// 		if keyword == "" {
// 			http.Error(w, "缺少搜索关键词", http.StatusBadRequest)
// 			return
// 		}

// 		// 调用搜索函数
// 		results, err := searchDocuments(keyword)
// 		if err != nil {
// 			log.Printf("搜索失败: %v", err)
// 			http.Error(w, "搜索失败", http.StatusInternalServerError)
// 			return
// 		}

// 		// 返回JSON结果
// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(results)
// 	})

// 	// 处理静态文件
// 	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
// 	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))

// 	// 重定向根路径到文档页面
// 	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
// 		if r.URL.Path == "/" {
// 			http.Redirect(w, r, "/doc/1", http.StatusFound)
// 			return
// 		}
// 		http.NotFound(w, r)
// 	})

// 	// 启动服务器
// 	port := ":8080"
// 	fmt.Printf("服务器启动在 http://localhost%s\n", port)
// 	log.Fatal(http.ListenAndServe(port, nil))
// }
