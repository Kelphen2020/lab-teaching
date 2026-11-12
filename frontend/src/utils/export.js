// 前端 CSV 导出工具
export function downloadCSV(filename, headers, rows) {
  const BOM = '\uFEFF'
  const headerLine = headers.join(',')
  const dataLines = rows.map(row =>
    headers.map(h => {
      const val = row[h] ?? ''
      const s = String(val)
      // 包含逗号、引号或换行时用双引号包裹
      if (s.includes(',') || s.includes('"') || s.includes('\n')) {
        return '"' + s.replace(/"/g, '""') + '"'
      }
      return s
    }).join(',')
  )
  const csv = BOM + headerLine + '\n' + dataLines.join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// 后端导出：直接打开下载链接
export function downloadBackendExport(url, token) {
  const a = document.createElement('a')
  a.href = url
  // 如果 API 需要 token，通过 query 参数传递（后端支持 ?token=xxx）
  if (token) {
    a.href += (url.includes('?') ? '&' : '?') + 'token=' + encodeURIComponent(token)
  }
  a.download = ''
  a.click()
}