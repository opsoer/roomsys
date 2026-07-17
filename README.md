# 圳好租 - 深圳公寓租赁管理系统

多租户 SaaS 财务管理平台，面向深圳本地合伙制公寓租赁业务。

## 功能特性

- **多租户架构**：单数据库 + `building_id` 行级隔离
- **角色权限**：超级管理员 / 楼栋管理员 / 普通管理员
- **房间管理**：状态流转（未出租→已出租→即将到期→已到期）
- **财务管理**：账单CRUD + 月度/年度统计 + 收支趋势
- **分红系统**：股东配置 + 分红计算 + 预测
- **待办任务**：到期退租自动创建任务
- **媒体管理**：房间图片/视频上传
- **时间模拟**：开发环境时间偏移测试

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25+ / Gin / GORM |
| 前端 | Vue 3 / Pinia / Vite 6 |
| UI | Element Plus + Vant 4（按需导入） |
| 数据库 | MySQL 5.7+ |
| 测试 | Vitest + Go testing |

## 快速开始

### 1. 数据库

```sql
CREATE DATABASE rental CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 2. 启动后端

```bash
cd server
go mod tidy
$env:DB_PASSWORD="root"  # 设置MySQL密码
go run main.go
```

### 3. 启动前端

```bash
cd web
npm install
npm run dev
```

### 4. 访问系统

- 首页：http://localhost:5173
- 管理后台：http://localhost:5173/login
- 账号：`admin / admin123`（超级管理员）

## 测试

```bash
# 前端测试
cd web && npm test

# 后端测试
cd server && go test ./utils/ -v
```

## 项目结构

```
roomsys/
├── server/                 # Go后端
│   ├── handlers/           # HTTP处理器
│   ├── services/           # 业务逻辑层
│   ├── models/             # 数据模型
│   └── utils/              # 工具函数
└── web/                    # Vue前端
    ├── src/
    │   ├── api/            # API调用
    │   ├── stores/         # Pinia状态
    │   ├── components/     # 组件
    │   └── views/          # 页面
    └── tests/              # 测试
```

## 文档

- [项目文档](项目文档.md) - 完整设计文档
- [待修改清单](待修改.md) - 问题追踪
- [回归检查报告](回归检查报告.md) - 修复验证
