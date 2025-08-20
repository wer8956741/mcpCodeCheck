#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');

// 获取二进制文件路径
function getBinaryPath() {
  const platform = os.platform();
  const arch = os.arch();
  
  let binaryName = 'lint-mcp';
  if (platform === 'win32') {
    binaryName += '.exe';
  }
  
  return path.join(__dirname, 'bin', binaryName);
}

// 启动 MCP 服务器
function startMCPServer() {
  const binaryPath = getBinaryPath();
  
  // 检查二进制文件是否存在
  const fs = require('fs');
  if (!fs.existsSync(binaryPath)) {
    console.error(`Error: Binary not found at ${binaryPath}`);
    console.error('Please run "npm run build" to compile the Go binary.');
    process.exit(1);
  }
  
  // 启动子进程
  const child = spawn(binaryPath, [], {
    stdio: 'inherit',
    env: process.env
  });
  
  // 处理进程退出
  child.on('exit', (code, signal) => {
    if (signal) {
      console.error(`lint-mcp was killed with signal ${signal}`);
      process.exit(1);
    } else if (code !== 0) {
      console.error(`lint-mcp exited with code ${code}`);
      process.exit(code);
    }
  });
  
  // 处理错误
  child.on('error', (err) => {
    console.error(`Failed to start lint-mcp: ${err.message}`);
    process.exit(1);
  });
  
  // 处理进程信号
  process.on('SIGINT', () => {
    child.kill('SIGINT');
  });
  
  process.on('SIGTERM', () => {
    child.kill('SIGTERM');
  });
}

// 如果直接运行此文件，启动服务器
if (require.main === module) {
  startMCPServer();
}

module.exports = {
  startMCPServer,
  getBinaryPath
};
