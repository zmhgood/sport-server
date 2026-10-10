# 指定版本发布约定

## 2026-10-10 发布记录

三个仓库的同名标签 `sport-release-20261010.1` 共同定义本次版本：
- sport-server：仅微信登录；用户/管理员鉴权隔离；家庭读取与目标成员权限；空手机号兼容；停止输出微信密钥与响应。
- sprot-adfront：生产路径 /ad/、路由刷新、登录表单校验、移除随机生成的活跃趋势。
- sport-mini：已发布源码中的界面美化、真实动作媒体、GIF/视频、头像持久化、仅微信登录和隐私政策。

本次线上程序先通过 SSH 发布，随后补齐 Git 记录；Git 提交不是一次新的应用部署。
后端配置、数据库、上传媒体不在发布包内，业务数据未同步。
小程序开发版本 1.0.20261010.1 已上传，审核/正式发布另行处理。

验证：后端全包测试（含本地 MySQL 回滚事务集成测试）与 go vet；Linux 构建；admin 生产构建与登录页浏览器验证；小程序语法和登录/媒体/头像模拟检查；线上 17 项 API 检查及公共健康、admin 资源访问。
验证限制：未完成真实设备微信登录/头像上传/视频播放及完整已登录 admin 浏览器回归；历史已暴露凭据的轮换仍需单独处理。
原始线上备份：/root/sport-release-20261010-1650/backup。
TLS 和 Cloudflare 配置独立保留，不属于三个源码仓库。

## 后续由服务器龙虾发布

1. 用户指定完整的 server/admin/mini 提交 SHA；没有指定版本时先确认，禁止默认部署 master 最新提交。
2. 先运行 `/opt/sport/verify-release.py SERVER_SHA ADMIN_SHA MINI_SHA`，验证提交可从 origin/master 到达，且包含本次修复基线。失败即停止。
3. 使用独立 checkout/worktree 构建。不要在 /opt/sport/server、admin、mini 现场目录执行 pull、reset、clean、checkout。
4. 后端先运行 go test ./...、go vet ./...，再 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build。集成测试只允许专用本地/测试数据库，禁止指向线上库。
5. admin 使用 npm ci 和 npm run build，核对 dist/index.html 的 /ad/assets/ 路径。任何取码、测试或构建失败都停止，禁止继续发布旧产物。
6. 保留 config/config.yaml -> config.local.yaml、私密配置、证书、Cloudflare、数据库、上传目录。不要运行 apply-config.sh，不同步本地动作数据，不修改 nginx 路由。
7. 备份运行二进制和 /var/www/admin，保存三 SHA、产物哈希及回滚位置。构建成功后原子替换 /opt/sport/server/main，仅用 systemctl restart dosport-backend.service 管理后端。
8. admin 在暂存目录保留旧 hashed assets 后叠加新 dist，再切换 /var/www/admin；不删除线上旧资源。静态文件发布通常无需重启 nginx。
9. 核对 systemd 状态、/health、公共 /api/health、/ad/login 与引用资源，并运行角色权限、短信停用和家庭越权回归。失败恢复备份并重新检查。
10. 记录实际运行二进制哈希、admin 资源哈希与三 SHA。mini 只留档源码，不由服务器脚本发布微信版本。

旧 deploy-backend.sh（kill/nohup）与 deploy-frontend.sh（拉取失败继续）停用。发布入口为本约定、版本校验器及明确的用户部署指令。
基线版本与现场产物记录保存在 /opt/sport/releases/current.json；它记录这次直接发布的产物，不能将后续重新构建的哈希误当成相同。

## 与线上龙虾核对后的补充

- 校验器的 fetch 允许更新 Git 对象、远端跟踪 refs 和标签；不改工作树、现场 HEAD、stash 或应用运行状态。现场目录仅作为运行态和既有 Git 对象来源，禁止原地构建、切分支、清理或恢复 stash。
- 入参是提交 SHA，不是注释标签对象 SHA；标签用 git rev-parse TAG^{commit} 解析。校验器检查修复基线和远端可达性；三仓库的接口兼容性仍需发布前核对，祖先关系不能检测后续人为撤销修复。
- baseline-20261010.json 是不可变的最低修复基线，current.json 是当前运行产物记录。后续成功发布才更新 current，并追加时间、操作人、命令、三 SHA、实际产物哈希、验证结果与确切回滚位置。
- /root/sport-release-20261010-1650/backup 是此次修复前的历史备份，不能当作下一次发布的默认回滚版本。下一次发布应重新备份当前 main 和 /var/www/admin，失败恢复本次新备份；admin-before-final* 是此次发布中间阶段，也不能默认认为是终版。
- 发布前后检查 config/config.yaml 的符号链接目标，以及私密配置文件哈希；只记录完整性结果，不输出文件内容。重新构建的产物允许因工具链差异产生不同哈希，应记录实际运行哈希，不能伪称等同本次 SSH 产物。
- 尚未实施自动构建/切换脚本；后续由龙虾依本约定执行明确版本发布。现有校验器只验证版本，不自动部署。
