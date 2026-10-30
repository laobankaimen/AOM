import { writeFileSync, chmodSync, readFileSync, readdirSync, copyFileSync, existsSync, unlinkSync } from 'node:fs';
import { createHash } from 'node:crypto';
const linux = 'release/aom-linux-amd64';const windows = 'release/aom-windows-amd64';
for (const [name, addr] of [['local', '127.0.0.1:8080'], ['lan', '0.0.0.0:8080']]) {
  const sh = `${linux}/start-${name}.sh`;
  const transport = name === 'lan' ? ' --https' : '';
  writeFileSync(sh, `#!/usr/bin/env bash\nset -e\ncd "$(dirname "$0")"\nexec ./aom --addr ${addr}${transport} "$@"\n`);chmodSync(sh, 0o755);
  writeFileSync(`${windows}/start-${name}.cmd`, ['@echo off','chcp 65001 >nul','cd /d "%~dp0"',`aom.exe --addr ${addr}${transport} %*`,'echo.','pause',''].join('\r\n'));
}
for (const [directory, binary] of [[linux, 'aom'], [windows, 'aom.exe']]) {
  for (const name of readdirSync('docs').filter(name => name.endsWith('.md'))) copyFileSync(`docs/${name}`, `${directory}/docs/${name}`);
  const obsoleteNames=['首次使用-三步初始化.md', '布局模板与初始化.md', '交互操作说明.md', '登录与数据保护.md', '微模块布局依据.md', '实施方案.md', '验收记录.md'];
  for(const name of obsoleteNames){const old=`${directory}/docs/${name}`;if(existsSync(old))unlinkSync(old)}
  writeFileSync(`${directory}/开始使用.txt`, 'AOM 机房数字孪生与运维系统\n\n应用处于开发阶段。\n\n1. 完整解压本地运行包。\n2. 本机运行start-local，浏览器访问 http://localhost:8080。\n3. 局域网运行start-lan，浏览器访问 https://服务器IP:8080。脚本会生成本地自签名证书，需先核对并信任data/server.crt；也可指定组织签发的证书。\n4. 首次使用从控制台复制初始化令牌，在页面创建管理员账号和密码。\n5. 以后登录需同一账号及密码；密码用于加密数据，丢失无法恢复。\n6. 数据保存到data/app.vault；停止程序后复制整个目录可备份或换机。\n7. 如导入现有app.db，原文件及旧备份仍是明文，核对后须妥善处理。\n\n登录后点击“初始化机房”，选择目标、上传总清单、预览后创建。\n见 docs/使用说明.md。\n登录、HTTPS、密码与备份说明见 docs/使用说明.md。\n');
  const hash = createHash('sha256').update(readFileSync(`${directory}/${binary}`)).digest('hex');
  writeFileSync(`${directory}/SHA256SUMS.txt`, `${hash}  ${binary}\n`);
}
