---
name: gorm-postgres
description: GORM v2 + PostgreSQL 核心 API 与最佳实践（连接池、模型、查询、事务、Hook、错误）
metadata:
  type: reference
---

# GORM v2 + PostgreSQL 速查

> 来源：[gorm.io/docs](https://gorm.io/docs/)、[github.com/go-gorm/gorm](https://github.com/go-gorm/gorm)、[gorm.io/driver/postgres](https://github.com/go-gorm/postgres)
> 本项目使用 **gorm.io/gorm v1.25.12** + **gorm.io/driver/postgres v1.5.9**。

---

## 1. 初始化

### 1.1 打开连接

```go
import (
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

dsn := "host=localhost user=app password=*** dbname=app port=5432 sslmode=disable TimeZone=Asia/Shanghai"
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    Logger:                 customLogger,      // 自定义 logger.Interface
    DisableForeignKeyConstraintWhenMigrating: true,
    NowFunc:                func() time.Time { return time.Now().UTC() },
    PrepareStmt:            true,              // 缓存 prepared statement
})
```

### 1.2 拿到底层 *sql.DB 配连接池

```go
sqlDB, err := db.DB()
sqlDB.SetMaxOpenConns(50)              // 最大打开连接
sqlDB.SetMaxIdleConns(10)              // 最大空闲
sqlDB.SetConnMaxLifetime(30 * time.Minute)
sqlDB.SetConnMaxIdleTime(5 * time.Minute)
```

规则：
- `MaxOpenConns` ≥ 工作线程数；太大打爆 PG `max_connections`。
- `MaxIdleConns` < `MaxOpenConns`，否则高并发时无法拉新连接。
- `ConnMaxLifetime` 建议 ≤ 30 min，避免 PG 中 `idle in transaction` 僵死。

### 1.3 Ping

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
if err := sqlDB.PingContext(ctx); err != nil {
    return fmt.Errorf("postgres ping: %w", err)
}
```

---

## 2. 模型定义

来源：[gorm.io/docs/models.html](https://gorm.io/docs/models.html)

### 2.1 GORM Tag 速查

```go
type User struct {
    ID        int64          `gorm:"primaryKey;column:id;autoIncrement"`
    Phone     string         `gorm:"column:phone_number;size:20;uniqueIndex:uk_phone"`
    Name      string         `gorm:"size:64;default:''"`
    Age       int            `gorm:"check:age >= 0"`
    Status    int8           `gorm:"default:0;index:idx_status"`
    Bio       string         `gorm:"type:text"`
    Metadata  datatypes.JSON `gorm:"type:jsonb"`           // pg jsonb
    ManagerID *int64         `gorm:"index"`
    CreatedAt time.Time      `gorm:"autoCreateTime"`
    UpdatedAt time.Time      `gorm:"autoUpdateTime"`
    DeletedAt gorm.DeletedAt `gorm:"index"`                // 软删
}

func (User) TableName() string { return "user" }
```

约定：
- 字段 `ID` 自动主键。
- `CreatedAt`/`UpdatedAt`/`DeletedAt` 自动维护时间。
- 列名默认 snake_case（`PhoneNumber → phone_number`）。
- 指针类型 = NULL 可空，非指针 = NOT NULL。

### 2.2 复合主键

```go
type UserRole struct {
    UserID int64 `gorm:"primaryKey"`
    RoleID int64 `gorm:"primaryKey"`
}
```

### 2.3 嵌入结构（类似继承）

```go
type BaseModel struct {
    ID        int64 `gorm:"primaryKey"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

type User struct {
    BaseModel
    Name string
}
```

### 2.4 软删除

`gorm.DeletedAt` 字段触发软删；查询自动加 `WHERE deleted_at IS NULL`。

```go
db.Delete(&user)                              // 软删
db.Unscoped().Delete(&user)                   // 真删
db.Unscoped().Where("name = ?", "x").Find(&list) // 含已删
```

### 2.5 PostgreSQL 专用类型

| 用途 | tag |
|---|---|
| JSON | `gorm:"type:jsonb"` + `datatypes.JSON` 或自定义 `Scanner/Valuer` |
| UUID | `gorm:"type:uuid;default:gen_random_uuid()"` |
| 数组 | `pq.StringArray` / 自定义 |
| 枚举 | `gorm:"type:varchar(20)"` + Go 枚举类型 |

---

## 3. 迁移（AutoMigrate）

来源：[gorm.io/docs/migration.html](https://gorm.io/docs/migration.html)

```go
err := db.AutoMigrate(&User{}, &Order{})
```

- ✅ 仅适合开发与小项目；不会删除列，不会改列类型。
- ❌ 生产强烈推荐手写迁移（[golang-migrate/migrate](https://github.com/golang-migrate/migrate) 或 [pressly/goose](https://github.com/pressly/goose)）。

---

## 4. 查询

来源：[gorm.io/docs/query.html](https://gorm.io/docs/query.html)

### 4.1 基础 CRUD

```go
// Create
db.Create(&user)
db.Select("Name", "Age").Create(&user)             // 只 insert 部分字段
db.Omit("Bio").Create(&user)

// Read
db.First(&user, 1)                                 // 主键
db.Take(&user)                                     // LIMIT 1
db.Last(&user)
db.Find(&users, []int{1, 2, 3})                    // 主键列表
db.Where("phone_number = ?", phone).First(&user)
db.Where(&User{Phone: phone}).First(&user)         // struct 条件
db.Where("name LIKE ?", "%x%").Find(&users)
db.Where("age BETWEEN ? AND ?", 18, 30).Find(&users)

// Update
db.Model(&user).Update("name", "new")             // 单字段
db.Model(&user).Updates(User{Name: "x", Age: 18})  // 非零字段
db.Model(&user).Updates(map[string]any{"name": "x"})
db.Model(&user).Select("name").Updates(...)        // 强制零值字段更新

// Delete
db.Delete(&user)
db.Where("status = ?", 0).Delete(&users)
```

### 4.2 上下文（必须）

```go
db.WithContext(ctx).Where(...).First(&user)
```

DAO 方法第一个参数始终是 `ctx`。

### 4.3 高级

```go
// Select 指定列
db.Select("id", "name").Find(&users)
db.Select("COALESCE(SUM(amount), 0)").Scan(&result)

// Group / Having
db.Model(&Order{}).
    Select("user_id, SUM(amount) AS total").
    Group("user_id").
    Having("SUM(amount) > ?", 1000).
    Scan(&results)

// Joins
db.Joins("JOIN user ON user.id = order.user_id").
    Where("user.status = ?", 0).
    Find(&orders)

db.Joins("Profile").Find(&users)        // 关系字段名（已 preload 等价）

// 子查询
db.Where("amount > (?)", db.Model(&Order{}).Select("AVG(amount)")).Find(&orders)

// CTE (v1.25+)
cte := db.Model(&Order{}).Where("status = ?", "paid")
db.Where("id IN (?)", cte).Find(&users)
```

### 4.4 锁

```go
db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, 1)  // SELECT ... FOR UPDATE
```

---

## 5. 预加载与关联

来源：[gorm.io/docs/preload.html](https://gorm.io/docs/preload.html)

### 5.1 定义关联

```go
type User struct {
    ID      int64
    Profile *Profile
    Orders  []Order
}

type Profile struct {
    UserID int64
    Bio    string
}

type Order struct {
    UserID int64
    Amount int64
}
```

### 5.2 预加载

```go
// 单层
db.Preload("Orders").Find(&users)

// 嵌套
db.Preload("Orders.Items").Find(&users)

// 条件预加载
db.Preload("Orders", "status = ?", "paid").Find(&users)

// 多字段
db.Preload("Profile").Preload("Orders").Find(&users)

// Joins 预加载（一条 SQL，关联列有索引时推荐）
db.Joins("Profile").Find(&users)
```

避免 N+1：能 Preload 就 Preload；非要遍历再查，改成 `IN (?)` 批量查。

---

## 6. 事务

来源：[gorm.io/docs/transactions.html](https://gorm.io/docs/transactions.html)

### 6.1 自动事务

```go
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&user).Error; err != nil { return err }
    if err := tx.Create(&order).Error; err != nil { return err }
    return nil
})
```

返回 `error` 时回滚；`nil` 时提交。**不要**在闭包内 commit/rollback，框架帮你做。

### 6.2 手动事务

```go
tx := db.Begin()
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
    }
}()

if err := tx.Create(&user).Error; err != nil {
    tx.Rollback()
    return err
}
return tx.Commit().Error
```

### 6.3 保存点（嵌套）

```go
db.Transaction(func(tx *gorm.DB) error {
    tx.Create(&user)
    return tx.Transaction(func(tx2 *gorm.DB) error {
        return tx2.Create(&order).Error  // 自动 savepoint
    })
})
```

### 6.4 隔离级别

```go
tx := db.Begin(&sql.TxOptions{Isolation: sql.LevelReadCommitted})
```

---

## 7. Hook

来源：[gorm.io/docs/hooks.html](https://gorm.io/docs/hooks.html)

```go
func (u *User) BeforeCreate(tx *gorm.DB) error {
    if u.Phone == "" {
        return errors.New("phone required")
    }
    return nil
}

func (u *User) AfterCreate(tx *gorm.DB) error {
    return publishEvent("user.created", u.ID)
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
    // tx.Statement.Schema 拿到当前操作的 schema
    return nil
}
```

Hook 列表：`BeforeSave / AfterSave`、`BeforeCreate / AfterCreate`、`BeforeUpdate / AfterUpdate`、`BeforeDelete / AfterDelete`、`AfterFind`。

Hook 错误会中止当前操作。**不要**在 hook 里发外部 HTTP/IO，绕开事务边界。

---

## 8. SQL 构建与 Raw

```go
// 表达式（拼接到 SQL）
db.Model(&user).Update("created_at", gorm.Expr("NOW()"))

// 原生 SQL
db.Raw("SELECT * FROM users WHERE name = ?", name).Scan(&user)
db.Exec("UPDATE users SET status = 0 WHERE last_login_at < ?", cutOff)

// OnConflict (PG: ON CONFLICT DO NOTHING / DO UPDATE)
db.Clauses(clause.OnConflict{DoNothing: true}).Create(&user)
db.Clauses(clause.OnConflict{
    Columns:   []clause.Column{{Name: "phone_number"}},
    DoUpdates: clause.AssignmentColumns([]string{"updated_at"}),
}).Create(&user)
```

---

## 9. 错误处理

来源：[gorm.io/docs/error_handling.html](https://gorm.io/docs/error_handling.html)

### 9.1 内置错误

```go
import "gorm.io/gorm"

errors.Is(err, gorm.ErrRecordNotFound)            // 未找到
errors.Is(err, gorm.ErrInvalidTransaction)         // 事务失效
errors.Is(err, gorm.ErrNotImplemented)
errors.Is(err, gorm.ErrMissingWhereClause)         // 危险：update/delete 无 where
errors.Is(err, gorm.ErrUnsupportedDriver)
errors.Is(err, gorm.ErrRegistered)                 // 已注册
errors.Is(err, gorm.ErrInvalidData)                // 数据非法
errors.Is(err, gorm.ErrInvalidSchema)
```

### 9.2 PG 唯一冲突

```go
import "github.com/jackc/pgx/v5/pgconn"

var pgErr *pgconn.PgError
if errors.As(err, &pgErr) {
    if pgErr.Code == "23505" { // unique_violation
        return ErrPhoneAlreadyUsed
    }
}
```

PG SQLSTATE 见 [PG 错误码表](https://www.postgresql.org/docs/current/errcodes-appendix.html)。

---

## 10. Logger 适配

来源：[gorm.io/docs/logger.html](https://gorm.io/docs/logger.html)

实现 `logger.Interface`：

```go
type Interface interface {
    LogMode(LogLevel) Interface
    Info(context.Context, string, ...interface{})
    Warn(context.Context, string, ...interface{})
    Error(context.Context, string, ...interface{})
    Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error)
}
```

慢查询阈值：`SlowThreshold > 200ms` 视为慢。建议 slog 字段名固定 `elapsed_ms / sql / rows / error`。

---

## 11. 性能与坑

### 11.1 N+1

```go
// ❌ N+1
for _, u := range users {
    db.Model(&Profile{}).Where("user_id = ?", u.ID).Find(&u.Profile)
}

// ✅ 一次
db.Preload("Profile").Find(&users)
```

### 11.2 批量插入

```go
const batchSize = 1000
db.CreateInBatches(users, batchSize)
```

### 11.3 不要忘记 `WithContext`

否则客户端断开后查询继续跑，浪费资源。

### 11.4 `Pluck` 提取单列

```go
var phones []string
db.Model(&User{}).Where("status = ?", 0).Pluck("phone_number", &phones)
```

### 11.5 软删查询要明确

`db.Find(&users)` 自动过滤 `deleted_at IS NULL`；要查所有需 `.Unscoped()`。

### 11.6 `gorm.io/datatypes` 常用类型

| 类型 | 说明 |
|---|---|
| `datatypes.JSON` | jsonb 字段 |
| `datatypes.Date` | 仅日期 |
| `datatypes.JSONMap / JSONSlice` | map/slice 形态 |
| `datatypes.NewJSONType` | 自定义 JSON 类型 |

---

## 12. 测试与 mocking

```go
// 方式 1：SQLite in-memory（注意 PG 特性不兼容）
db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

// 方式 2：testcontainers-go 跑真实 PG
pgC, _ := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
    Image: "postgres:16-alpine",
    ...
})

// 方式 3：接口层 mock（不依赖 DB）
type UserDAOInterface interface {
    GetByID(ctx context.Context, id int64) (*User, error)
}
```

---

## 13. 官方推荐 vs 反模式

| ❌ | ✅ |
|---|---|
| 直接 `db.First(&u, id)` 不传 ctx | `db.WithContext(ctx).First(...)` |
| AutoMigrate 在生产 | 手写迁移 |
| 在 handler/service 写裸 SQL | 走 DAO 层 |
| `db.Where(fmt.Sprintf("name = '%s'", n))` 拼接 | `?` 占位符 |
| update 不加 Where | 必须有 where，否则 `ErrMissingWhereClause` |
| 在 hook 发 HTTP | 用 outbox / async worker |
| `gorm:"-"` 字段又参与 Save | 用 `Select`/`Omit` 显式控制 |

---

## 相关资料链接

- [GORM v2 官方文档](https://gorm.io/docs/)
- [GORM GitHub](https://github.com/go-gorm/gorm)
- [GORM Postgres Driver](https://github.com/go-gorm/postgres)
- [PG 错误码](https://www.postgresql.org/docs/current/errcodes-appendix.html)
- [datatypes](https://github.com/go-gorm/datatypes)
