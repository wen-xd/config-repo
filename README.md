# AccessKey System with RAM Accounts and Role-Based Access Control

这是一个基于Go语言实现的AccessKey系统，支持多RAM账号和基于角色的权限控制。系统使用HMAC-SHA256签名机制来验证API请求的合法性。

## 功能特点

- 支持主账号创建多个RAM子账号
- 基于角色的权限控制系统
- 使用HMAC-SHA256进行请求签名和验证
- 灵活的权限管理，支持JSON格式的权限定义
- 支持访问密钥的生命周期管理（创建、验证、过期）

## 数据库结构

系统使用以下数据表：

1. `access_keys` - 存储访问密钥信息
2. `roles` - 定义角色及其权限
3. `users` - 用户信息
4. `access_key_roles` - 访问密钥与角色的关联关系

## 使用方法

### 初始化数据库连接

```go
err := accesskey.InitDB("user:password@tcp(localhost:3306)/accesskey_db")
if err != nil {
    // 处理错误
}
```

### 创建访问密钥

```go
// 创建权限JSON
permissions := map[string]interface{}{
    "resources": []string{"api/v1/users/*", "api/v1/products/read"},
    "actions":   []string{"GET", "POST"},
    "effect":    "allow",
}

// 转换为JSON字符串
permissionsJSON, _ := json.Marshal(permissions)

// 为用户创建访问密钥
id, secret, err := accesskey.CreateAccessKey(userId, string(permissionsJSON))
```

### 分配角色给访问密钥

```go
err := accesskey.AssignRoleToAccessKey(accessKeyID, roleID)
```

### 签名HTTP请求

```go
// 创建HTTP请求
req, _ := http.NewRequest("GET", "http://example.com/api/v1/users", nil)

// 签名请求
accesskey.SignRequest(req, accessKeyID, accessKeySecret, nil)
```

### 验证请求签名

```go
// 在服务器端验证签名
valid, err := accesskey.VerifyRequestSignature(req, nil)
if err != nil || !valid {
    // 处理无效签名
}
```

### 使用中间件验证请求

```go
// 创建签名验证中间件
middleware := accesskey.CreateMiddleware()

// 在HTTP服务器中使用中间件
http.Handle("/api/", middleware(apiHandler))
```

## 权限管理

权限使用JSON格式定义，例如：

```json
{
    "resources": ["api/v1/users/*", "api/v1/products/read"],
    "actions": ["GET", "POST"],
    "effect": "allow"
}
```

## 安全建议

1. 妥善保管AccessKey Secret，不要在客户端代码中硬编码
2. 定期轮换访问密钥
3. 遵循最小权限原则，只分配必要的权限
4. 使用HTTPS传输所有API请求
5. 实现请求重放保护（可使用时间戳和nonce）

## 扩展功能

- 支持多种认证方式（如JWT、OAuth等）
- 实现访问密钥的自动轮换
- 添加审计日志功能
- 实现基于IP地址的访问控制
- 支持临时访问凭证