## Powershell

function sha256sum($str) {
    $sha256 = new-object -TypeName System.Security.Cryptography.SHA256Managed
    $utf8   = new-object -TypeName System.Text.UTF8Encoding
    $hash   = [System.BitConverter]::ToString($sha256.ComputeHash($utf8.GetBytes($str)))
    return $hash.replace('-','').toLower()
}


get-content -Encoding ASCII scripts/explan_udata.sql | sqlite3 explan_udata.sqlite3
$sql = $(sqlite3 explan_udata.sqlite3 "select sql from sqlite_schema;") -join "`n"
$sql = $sql + "`n"
## $sql | Out-File -Encoding ASCII xxx.sql
sha256sum $sql
