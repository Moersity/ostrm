'use strict';
const repository = 'https://github.com/Moersity/ostrm';
const releaseURL = `${repository}/releases/latest`;
let release;
function updateDownloads() {
  for (const link of document.querySelectorAll('.download-link')) {
    const os = link.dataset.os;
    const arch = document.querySelector(`select[data-os="${os}"]`).value;
    const version = release.tag_name.replace(/^v/, '');
    const extension = os === 'windows' ? '_setup.exe' : os === 'darwin' ? '.pkg' : '.tar.gz';
    const name = `ostrm_${version}_${os}_${arch}${extension}`;
    const asset = release.assets.find(asset => asset.name === name);
    // Download only the expected asset from this repository and release.
    const expected = `${repository}/releases/download/${encodeURIComponent(release.tag_name)}/${name}`;
    link.href = asset?.browser_download_url === expected ? expected : releaseURL;
    link.textContent = asset?.browser_download_url === expected ? `下载 ${version} ↓` : '前往 Release 查找安装包 ↗';
    link.setAttribute('aria-label', `${os} ${arch} ${link.textContent}`);
  }
}
for (const select of document.querySelectorAll('select[data-os]')) {
  select.addEventListener('change', () => { if (release) updateDownloads(); });
}
async function loadRelease() {
  try {
    const response = await fetch('https://api.github.com/repos/Moersity/ostrm/releases/latest', {signal: AbortSignal.timeout(8000)});
    if (!response.ok) throw new Error('Release unavailable');
    const data = await response.json();
    if (data.draft || data.prerelease || !/^v?\d+\.\d+\.\d+$/.test(data.tag_name) || !Array.isArray(data.assets)) throw new Error('Invalid release');
    release = data;
    document.querySelector('#release-label').textContent = `最新正式版 ${data.tag_name} ↗`;
    updateDownloads();
  } catch {
    document.querySelector('#download-status').textContent = '暂时无法读取版本信息，可直接前往 GitHub Releases 选择对应安装包。';
  }
}
loadRelease();
