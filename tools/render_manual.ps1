$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$source = (Get-ChildItem -LiteralPath $root -Filter 'ZZZ_Multi_Agent_Drive_Optimizer_v2.1.0_*.md' | Select-Object -First 1).FullName
if (-not $source) { throw 'Manual Markdown source was not found.' }
$target = [System.IO.Path]::ChangeExtension($source, '.html')

function Inline-Markdown([string]$text) {
    $value = [System.Net.WebUtility]::HtmlEncode($text)
    $value = [regex]::Replace($value, '``([^`]+)``', '<code>$1</code>')
    $value = [regex]::Replace($value, '`([^`]+)`', '<code>$1</code>')
    $value = [regex]::Replace($value, '\*\*([^*]+)\*\*', '<strong>$1</strong>')
    $value = [regex]::Replace($value, '\[([^]]+)\]\((https?://[^)]+)\)', '<a href="$2">$1</a>')
    return $value
}

$lines = Get-Content -LiteralPath $source -Encoding UTF8
$body = [System.Text.StringBuilder]::new()
$paragraph = [System.Collections.Generic.List[string]]::new()
$table = [System.Collections.Generic.List[string]]::new()
$code = [System.Collections.Generic.List[string]]::new()
$listType = $null
$inCode = $false

function Add-Line([string]$value) { [void]$body.AppendLine($value) }
function Flush-Paragraph {
    if ($paragraph.Count) { Add-Line ('<p>' + (Inline-Markdown ($paragraph -join ' ')) + '</p>'); $paragraph.Clear() }
}
function Flush-List {
    if ($script:listType) { Add-Line ("</$script:listType>"); $script:listType = $null }
}
function Flush-Table {
    if (-not $table.Count) { return }
    $rows = foreach ($line in $table) { ,(($line.Trim().Trim('|') -split '\|') | ForEach-Object { $_.Trim() }) }
    Add-Line ('<table><thead><tr>' + (($rows[0] | ForEach-Object { '<th>' + (Inline-Markdown $_) + '</th>' }) -join '') + '</tr></thead><tbody>')
    for ($i = 2; $i -lt $rows.Count; $i++) { Add-Line ('<tr>' + (($rows[$i] | ForEach-Object { '<td>' + (Inline-Markdown $_) + '</td>' }) -join '') + '</tr>') }
    Add-Line '</tbody></table>'; $table.Clear()
}

foreach ($line in $lines) {
    if ($line.StartsWith('```')) {
        Flush-Paragraph; Flush-List; Flush-Table
        if ($inCode) { Add-Line ('<pre><code>' + [System.Net.WebUtility]::HtmlEncode(($code -join "`n")) + '</code></pre>'); $code.Clear() }
        $inCode = -not $inCode; continue
    }
    if ($inCode) { $code.Add($line); continue }
    if ($line.StartsWith('|')) { Flush-Paragraph; Flush-List; $table.Add($line); continue }
    Flush-Table
    if ([string]::IsNullOrWhiteSpace($line)) { Flush-Paragraph; Flush-List; continue }
    if ($line -match '^(#{1,3})\s+(.+)$') { Flush-Paragraph; Flush-List; $level=$matches[1].Length; Add-Line ("<h$level>" + (Inline-Markdown $matches[2]) + "</h$level>"); continue }
    if ($line.StartsWith('> ')) { Flush-Paragraph; Flush-List; Add-Line ('<blockquote>' + (Inline-Markdown $line.Substring(2)) + '</blockquote>'); continue }
    if ($line -match '^\s*(?:([-*])|(\d+)\.)\s+(.+)$') {
        Flush-Paragraph; $wanted = if ($matches[1]) {'ul'} else {'ol'}
        if ($listType -ne $wanted) { Flush-List; Add-Line "<$wanted>"; $listType=$wanted }
        Add-Line ('<li>' + (Inline-Markdown $matches[3]) + '</li>'); continue
    }
    $paragraph.Add($line.Trim())
}
Flush-Paragraph; Flush-List; Flush-Table

$style = ':root{color-scheme:light;--ink:#172033;--line:#d8dfeb;--soft:#eef6fc}*{box-sizing:border-box}body{margin:0;background:#edf1f7;color:var(--ink);font:16px/1.72 "Segoe UI Variable Text","Microsoft YaHei UI","Microsoft YaHei",sans-serif}main{max-width:980px;margin:28px auto;padding:52px 64px;background:#fff;box-shadow:0 12px 40px rgba(22,33,55,.12);border-radius:16px}h1{font-size:32px;margin:0 0 24px;color:#10243e;border-bottom:3px solid #5ba7d6;padding-bottom:14px}h2{font-size:24px;margin:42px 0 14px;color:#153d63;border-bottom:1px solid var(--line);padding-bottom:8px}h3{font-size:19px;margin:28px 0 10px;color:#20527d}p{margin:9px 0}ul,ol{margin:8px 0 14px;padding-left:28px}li{margin:5px 0}a{color:#0969a9;text-decoration:none}code{font:14px/1.5 Consolas,"Cascadia Mono",monospace;background:#edf2f7;border-radius:5px;padding:2px 5px}pre{overflow:auto;background:#111827;color:#e7eefc;border-radius:10px;padding:15px 18px;margin:12px 0 18px}pre code{background:none;padding:0;color:inherit}blockquote{margin:18px 0;padding:12px 16px;border-left:4px solid #5ba7d6;background:var(--soft);color:#34465e;border-radius:0 8px 8px 0}table{width:100%;border-collapse:collapse;margin:14px 0 22px;font-size:14px}th,td{padding:9px 11px;border:1px solid var(--line);text-align:left}th{background:#eaf3fa;color:#173e61}tr:nth-child(even) td{background:#f8fafc}@media print{body{background:#fff;font-size:11pt}main{max-width:none;margin:0;padding:0;box-shadow:none}h1{font-size:24pt}h2{font-size:17pt;break-after:avoid}h3{font-size:14pt;break-after:avoid}table,pre,blockquote{break-inside:avoid}@page{size:A4;margin:16mm 15mm}}'
$document = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>ZZZ Multi-Agent Drive Optimizer v2.1.0 使用说明</title><style>' + $style + '</style></head><body><main>' + $body.ToString() + '</main></body></html>'
[System.IO.File]::WriteAllText($target, $document, [System.Text.UTF8Encoding]::new($false))
