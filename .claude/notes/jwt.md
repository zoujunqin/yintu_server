---
name: jwt
description: golang-jwt/jwt v5 核心 API 与最佳实践（签发、解析、Claims、刷新、中间件）
metadata:
  type: reference
---

# JWT (golang-jwt/jwt v5) 速查

> 来源：[github.com/golang-jwt/jwt](https://github.com/golang-jwt/jwt)、[pkg.go.dev/github.com/golang-jwt/jwt/v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)、[auth0 JWT intro](https://jwt.io/introduction)
> 本项目使用 **github.com/golang-jwt/jwt/v5 v5.3.1**。

---

## 1. 三段式结构

```
header.payload.signature
```

- `header`：算法 + 类型（`{"alg":"HS256","typ":"JWT"}`）
- `payload`：声明（claims），明文 base64url，**不加密**
- `signature`：HMAC/RSASSA 签名，防篡改

❗ JWT 不应携带敏感信息（PII、密码）；payload 可被任何人解码。

---

## 2. Claims 模型

来源：[pkg.go.dev/.../jwt/v5#RegisteredClaims](https://pkg.go.dev/github.com/golang-jwt/jwt/v5#RegisteredClaims)

### 2.1 标准注册字段（RegisteredClaims）

| 字段 | 含义 |
|---|---|
| `iss` | Issuer，签发方 |
| `sub` | Subject，用户 ID |
| `aud` | Audience，受众 |
| `exp` | Expiration，过期时间 |
| `nbf` | Not Before，生效时间 |
| `iat` | Issued At，签发时间 |
| `jti` | JWT ID，唯一 ID（防重放） |

### 2.2 自定义 Claims

```go
type Claims struct {
    UID         int64  `json:"uid"`
    PhoneNumber string `json:"phoneNumber"`
    jwt.RegisteredClaims
}
```

### 2.3 Claims 类型 vs 接口

v5 推荐用结构体（值/指针），解析走类型断言。

---

## 3. 签发 token

### 3.1 HMAC (HS256/HS384/HS512)

```go
import "github.com/golang-jwt/jwt/v5"

claims := Claims{
    UID:         uid,
    PhoneNumber: phone,
    RegisteredClaims: jwt.RegisteredClaims{
        Issuer:    "spring-slumber",
        Subject:   fmt.Sprintf("%d", uid),
        Audience:  jwt.ClaimStrings{"app"},
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
        IssuedAt:  jwt.NewNumericDate(time.Now()),
        NotBefore: jwt.NewNumericDate(time.Now()),
        ID:        uuid.NewString(),  // jti
    },
}

token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
signed, err := token.SignedString([]byte(secret))  // secret ≥ 32 bytes
```

### 3.2 RSA (RS256)

```go
signer, _ := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
signed, _ := token.SignedString(signer)
```

### 3.3 ECDSA (ES256)

```go
signer, _ := jwt.ParseECPrivateKeyFromPEM(privateKeyPEM)
signed, _ := token.SignedString(signer)
```

### 3.4 EdDSA (Ed25519)

```go
signer, _ := jwt.ParseEdPrivateKeyFromPEM(privateKeyPEM)
signed, _ := token.SignedString(signer)
```

### 3.5 Secret 安全

- HS256 secret ≥ 32 字节随机（`crypto/rand`）。
- 不要把 secret 写进 git / 日志。
- 生产密钥应从 KMS / Vault 拉，不放 env。

---

## 4. 解析与验证

### 4.1 基础解析

```go
claims := &Claims{}
token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
    // 1. 显式校验签名算法，防 alg=none 攻击
    if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
    }
    return secret, nil
})
```

### 4.2 错误判定（v5 错误体系）

v5 把所有错误归一为可 `errors.Is` 的哨兵：

```go
import "github.com/golang-jwt/jwt/v5"

switch {
case errors.Is(err, jwt.ErrTokenMalformed):
    // token 不是合法 JWT 格式
case errors.Is(err, jwt.ErrTokenSignatureInvalid):
    // 签名错
case errors.Is(err, jwt.ErrTokenExpired):
    // 过期
case errors.Is(err, jwt.ErrTokenNotValidYet):
    // nbf 未到
case errors.Is(err, jwt.ErrTokenInvalidAudience):
    // aud 不匹配
case errors.Is(err, jwt.ErrTokenInvalidIssuer):
    // iss 不匹配
case errors.Is(err, jwt.ErrTokenInvalidId):
    // jti 校验失败（v5.3+）
case errors.Is(err, jwt.ErrTokenInvalidClaims):
    // claims 类型不对
case errors.Is(err, jwt.ErrTokenInvalidSubject),
    errors.Is(err, jwt.ErrTokenInvalidIssuedAt):
    // 其它标准字段不匹配
}
```

### 4.3 加上 audience/issuer 校验

```go
jwt.ParseWithClaims(tokenStr, claims, keyFunc,
    jwt.WithIssuer("spring-slumber"),
    jwt.WithAudience("app"),
    jwt.WithExpirationRequired(),
    jwt.WithLeeway(30 * time.Second),
    jwt.WithValidMethods([]string{"HS256"}),
)
```

`WithValidMethods` 强烈推荐，防 `alg: none` 与算法替换。

### 4.4 完整模板

```go
token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, keyFunc,
    jwt.WithValidMethods([]string{"HS256"}),
    jwt.WithIssuer(cfg.Issuer),
    jwt.WithExpirationRequired(),
    jwt.WithLeeway(30*time.Second),
)
if err != nil {
    return nil, err
}
if !token.Valid {
    return nil, errors.New("invalid token")
}
```

---

## 5. 刷新（refresh）

### 5.1 双 token 模式

- **Access Token**：寿命短（15min ~ 2h），访问 API。
- **Refresh Token**：寿命长（7d ~ 30d），仅用于换新 access。

```go
// 登录返回两条
type LoginResult struct {
    AccessToken  string    `json:"accessToken"`
    RefreshToken string    `json:"refreshToken"`
    ExpiresAt    time.Time `json:"expiresAt"`
}
```

### 5.2 refresh 端点流程

```text
client → POST /api/v1/auth/refresh  {refreshToken: "..."}
server → 校验 jti 是否在黑名单 / 已撤销
server → 校验签名 / 过期
server → 取 sub，查 user
server → 签新 access，**旋转** refreshToken（撤销旧 refresh 的 jti）
server → 返回新 pair
```

刷新 token 应：
- 一次性（旋转）；
- 存入 DB（hash + jti），登出/改密时全量撤销；
- UA/IP 变化检测（高级场景）。

### 5.3 续期 vs 过期区别

```go
if errors.Is(err, jwt.ErrTokenExpired) {
    // 允许客户端用 refresh 换新 access
}
```

---

## 6. Gin 中间件

```go
import (
    "strings"
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
)

const ctxUIDKey = "uid"

func Auth(issuer *auth.Issuer) gin.HandlerFunc {
    return func(c *gin.Context) {
        h := c.GetHeader("Authorization")
        if !strings.HasPrefix(h, "Bearer ") {
            response.Problem(c, 401, "unauthorized", "missing bearer token")
            c.Abort()
            return
        }
        tokenStr := strings.TrimPrefix(h, "Bearer ")

        claims, err := issuer.Parse(tokenStr)
        if err != nil {
            response.Problem(c, 401, "invalid_token", err.Error())
            c.Abort()
            return
        }
        c.Set(ctxUIDKey, claims.UID)
        c.Next()
    }
}

// 取值
uid, _ := c.Get(ctxUIDKey)
uidInt := uid.(int64)
```

注意：401 vs 403：
- 401 Unauthorized：未携带有效凭证。
- 403 Forbidden：已认证但无权限。

---

## 7. 撤销（黑名单）

JWT 本身无法撤销（无状态）。可选方案：

1. **服务端会话侧**：登录态存 Redis，token 仅是「凭证 ID」。
2. **jti 黑名单**：注销/改密时把 jti 写 Redis，TTL = token 剩余寿命。

```go
// 注销
func Logout(jti string, ttl time.Duration) error {
    return redis.Set(ctx, "jwt:revoked:"+jti, "1", ttl).Err()
}

// 校验
if v, _ := redis.Get(ctx, "jwt:revoked:"+claims.ID).Result(); v != "" {
    return ErrTokenRevoked
}
```

---

## 8. 安全要点

来源：[OWASP JWT Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html)、[auth0 docs](https://auth0.com/docs/secure/tokens)

| ❌ 反模式 | ✅ 推荐 |
|---|---|
| `alg: none` | 服务端固定算法白名单 |
| HS256 secret < 32B | ≥ 32B 随机 |
| 不校验 exp | 必加 `WithExpirationRequired()` |
| 不校验 iss/aud | 显式 `WithIssuer/WithAudience` |
| JWT 存敏感数据 | 仅放 user_id；敏感数据查 DB |
| localStorage 存 JWT | httpOnly cookie + CSRF token |
| 单一长寿命 token | access + refresh 分离 |
| 不限制 token 大小 | header ≤ 8KB（cookie 限制） |
| 同 secret 多服务复用 | 每服务独立 secret |

---

## 9. 测试技巧

```go
func TestIssueAndParse(t *testing.T) {
    iss := auth.NewIssuer(config.JWTConfig{
        Secret:   "this-is-a-test-secret-with-32+chars",
        Issuer:   "test",
        ExpiresIn: time.Minute,
    })
    token, exp, err := iss.Issue(42, "13800000000")
    require.NoError(t, err)
    require.True(t, exp.After(time.Now()))

    claims, err := iss.Parse(token)
    require.NoError(t, err)
    require.Equal(t, int64(42), claims.UID)
}
```

生成 secret：
```bash
openssl rand -base64 48
```

---

## 10. 官方推荐 vs 反模式

| ❌ | ✅ |
|---|---|
| `jwt.Parse(...)` 不带算法校验 | `jwt.WithValidMethods` 固定白名单 |
| `token.Valid` 直接当 bool 用，不检查 err | err 优先 |
| 业务字段放自定义 Claim 后忘了 json tag | 显式 `json:"uid"` |
| refresh token 永不过期 | 14d 以内 + 旋转 |
| 把 secret 放在 git | KMS / env / Vault |
| 解析失败不区分原因，全返 401 | 区分 `ErrTokenExpired` 引导客户端刷新 |

---

## 11. v4 → v5 迁移要点

来源：[github.com/golang-jwt/jwt/releases/tag/v5.0.0](https://github.com/golang-jwt/jwt/releases/tag/v5.0.0)

- import 路径：`github.com/dgrijalva/jwt-go` → `github.com/golang-jwt/jwt/v5`。
- `jwt.StandardClaims` → `jwt.RegisteredClaims`。
- 错误处理：`jwt.ValidationError` → `errors.Is(err, jwt.Err...)`。
- `jwt.TimeFunc` → `jwt.TimeNow` 改名为 `jwt.NewNumericDate` 配合 `time.Time`。
- `jwt.MapClaims` 仍可用，但推荐结构体。

---

## 相关资料链接

- [jwt-go v5 GitHub](https://github.com/golang-jwt/jwt)
- [pkg.go.dev jwt/v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)
- [jwt.io 调试器](https://jwt.io)
- [OWASP JWT Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html)
- [RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519)
