'use strict';

const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { pipeline } = require('stream/promises');

module.exports = async function afterPack(context) {
  if (process.platform !== 'darwin') {
    return;
  }

  const appInfo = context.appInfo || (context.packager && context.packager.appInfo);
  const productName = appInfo && (appInfo.productFilename || appInfo.productName) ? (appInfo.productFilename || appInfo.productName) : 'App';
  let appPath = path.join(context.appOutDir, `${productName}.app`);
  if (!fs.existsSync(appPath)) {
    const candidates = fs.readdirSync(context.appOutDir).filter((name) => name.endsWith('.app'));
    if (candidates.length > 0) {
      appPath = path.join(context.appOutDir, candidates[0]);
    }
  }

  if (!fs.existsSync(appPath)) {
    throw new Error(`Failed to locate app bundle in ${context.appOutDir}`);
  }

  const xattrResult = spawnSync('xattr', ['-cr', appPath], { stdio: 'inherit' });
  if (xattrResult.status !== 0) {
    throw new Error('Failed to clear extended attributes before signing.');
  }

  const findResult = spawnSync('find', [appPath, '-name', '._*', '-delete'], { stdio: 'inherit' });
  if (findResult.status !== 0) {
    throw new Error('Failed to remove AppleDouble files before signing.');
  }

  const binaries = collectMacOsBinaries(appPath);
  for (const binaryPath of binaries) {
    await rewriteFile(binaryPath);
  }
};

function collectMacOsBinaries(appPath) {
  const results = [];

  addFilesInDir(path.join(appPath, 'Contents', 'MacOS'), results);

  const frameworksDir = path.join(appPath, 'Contents', 'Frameworks');
  if (!fs.existsSync(frameworksDir)) {
    return results;
  }

  for (const entry of fs.readdirSync(frameworksDir)) {
    if (!entry.endsWith('.app')) {
      continue;
    }
    const macosDir = path.join(frameworksDir, entry, 'Contents', 'MacOS');
    addFilesInDir(macosDir, results);
  }

  return results;
}

function addFilesInDir(dirPath, results) {
  if (!fs.existsSync(dirPath)) {
    return;
  }

  for (const name of fs.readdirSync(dirPath)) {
    const fullPath = path.join(dirPath, name);
    const stat = fs.lstatSync(fullPath);
    if (stat.isFile()) {
      results.push(fullPath);
    }
  }
}

async function rewriteFile(filePath) {
  const stat = fs.statSync(filePath);
  const tmpPath = `${filePath}.clean`;

  await pipeline(
    fs.createReadStream(filePath),
    fs.createWriteStream(tmpPath, { mode: stat.mode })
  );

  fs.chmodSync(tmpPath, stat.mode);
  fs.renameSync(tmpPath, filePath);
}
