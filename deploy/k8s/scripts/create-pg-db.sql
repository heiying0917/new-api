-- Tokenki 数据库初始化
--
-- ⚠️ 重要：腾讯云 PG 实例没有外网地址 + 控制台只有 GUI，**不能整文件执行**。
--          下面是分步流程：
--          - 步骤 1 / 2：必须走腾讯云 PG 控制台 GUI（账户管理 + 数据库管理）
--          - 步骤 3：可选兜底 SQL，仅在控制台 owner 设置异常时才需要执行（路径见文末）
--
-- 执行完后：把账户密码填入 deploy/k8s/manifests/prod/tokenki/secret.yaml 的 SQL_DSN

-- =========================================================================
-- 步骤 1（控制台 GUI 必走）：创建账户
-- =========================================================================
-- 腾讯云控制台 → 云数据库 → PostgreSQL → 实例 → 账户管理 → 创建账户
--   账户名：     tokenki_app
--   密码：       3jygDZ6b8wflMKsuGnV+RhrmYUW5qIiX
--   账户类型：   普通账户（不要给 superuser）
--   主机：       %（默认允许内网所有 IP 连接）
--   权限：       不勾"全部数据库"，留空，等步骤 2 建库时绑定
--
-- 对应等价 SQL（仅参考，控制台 GUI 不接受直接执行）：
--   CREATE USER tokenki_app WITH PASSWORD '3jygDZ6b8wflMKsuGnV+RhrmYUW5qIiX';

-- =========================================================================
-- 步骤 2（控制台 GUI 必走）：创建数据库
-- =========================================================================
-- 腾讯云控制台 → 云数据库 → PostgreSQL → 实例 → 数据库管理 → 创建数据库
--   数据库名：   tokenki
--   字符集：     UTF8
--   Owner：      tokenki_app   ← 关键，owner 设对了就不需要额外的 schema GRANT
--   Locale：     默认（C 或 en_US.UTF-8）
--   模板：       template0（PG 15+ 推荐，避免 collation 继承问题）
--
-- 对应等价 SQL（仅参考）：
--   CREATE DATABASE tokenki OWNER tokenki_app TEMPLATE template0 ENCODING 'UTF8';

-- =========================================================================
-- 步骤 3（兜底，通常不需要）：在 tokenki 库内补 schema 权限
-- =========================================================================
-- PG 18 行为：schema public 的 owner 默认 = 数据库 owner，PUBLIC 角色只有
--            USAGE 没 CREATE。tokenki_app 作为 database owner 已自动持有
--            schema public 全权限，**这一步不必跑**。
--
-- 仅当应用启动后报错类似 "permission denied for schema public" 时再跑
-- （说明步骤 2 创建库时 Owner 没设对，需要先在控制台修正 Owner 再补 GRANT）：
--
-- 1) 怎么连：腾讯云 PG 没有外网，必须从同 VPC 的 TKE 集群内连。
--    临时起一个 psql Pod（一次性容器，跑完即删；镜像版本对齐 PG 18）：
--
--      kubectl run pg-shell -n tokenki --rm -it --restart=Never \
--        --image=postgres:18-alpine -- \
--        psql "postgresql://tokenki_app:3jygDZ6b8wflMKsuGnV+RhrmYUW5qIiX@<PG_HOST>:5432/tokenki?sslmode=require"
--
--    （把 <PG_HOST> 替换为腾讯云 PG 实例的内网 IP）
--
-- 2) 进入 psql 后执行：

GRANT ALL ON SCHEMA public TO tokenki_app;

-- 注意：ALTER SCHEMA public OWNER TO ... 需要 superuser 权限，
--      腾讯云 root 不是真 superuser，**不要执行**，会报错。

-- =========================================================================
-- 步骤 4（控制台验证）：检查账户与库已创建
-- =========================================================================
-- 腾讯云控制台 → 账户管理 → 列表应能看到 tokenki_app
-- 腾讯云控制台 → 数据库管理 → 列表应能看到 tokenki（Owner=tokenki_app）
