# 本地测试数据库

使用本地 Docker Engine 和 `postgres:16-alpine` 启动 Management 测试数据库。容器端口只绑定 `127.0.0.1`，数据保存在 Docker 命名卷中。

```sh
cp deployments/.env.example deployments/.env
```

编辑 `deployments/.env`，为 `VELOTRUST_TEST_DB_PASSWORD` 设置本地测试密码，然后启动数据库：

```sh
docker compose --env-file deployments/.env -f deployments/postgres-test.compose.yml up -d
docker compose --env-file deployments/.env -f deployments/postgres-test.compose.yml ps
```

Management 的连接字符串格式为：

```text
postgres://velotrust:<VELOTRUST_TEST_DB_PASSWORD>@127.0.0.1:5432/velotrust?sslmode=disable
```

将密码替换为 `.env` 中的值后，可执行数据库迁移：

```sh
export DATABASE_DSN='postgres://velotrust:<password>@127.0.0.1:5432/velotrust?sslmode=disable'
go run ./cmd/management migrate --config management/configs/management.example.toml
```

停止容器但保留数据：

```sh
docker compose --env-file deployments/.env -f deployments/postgres-test.compose.yml down
```

删除容器及测试数据卷：

```sh
docker compose --env-file deployments/.env -f deployments/postgres-test.compose.yml down -v
```

`deployments/.env` 已被根目录 `.gitignore` 忽略，不要提交本地密码。
