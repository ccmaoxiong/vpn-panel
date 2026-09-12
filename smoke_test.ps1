$ErrorActionPreference = "Stop"
$base = "http://127.0.0.1:5097"
$s = New-Object Microsoft.PowerShell.Commands.WebRequestSession

function Api($method, $path, $body) {
    $params = @{ Uri = "$base$path"; Method = $method; WebSession = $s; UseBasicParsing = $true; TimeoutSec = 15 }
    if ($null -ne $body) {
        $params.ContentType = "application/json"
        $params.Body = ($body | ConvertTo-Json -Compress)
    }
    $r = Invoke-WebRequest @params
    return ($r.Content | ConvertFrom-Json)
}

Invoke-WebRequest -Uri "$base/login" -Method POST -Body @{username='admin';password='admin123'} -WebSession $s -UseBasicParsing -MaximumRedirection 5 | Out-Null

$checks = @()

$r = Api "POST" "/api/settings" @{ site_name="TestPanelSite"; sub_domain="sub.example.com"; traffic_reset_cycle="weekly" }
$checks += "settings save: $($r.success)"

$r = Api "POST" "/api/settings" @{ new_password="newpass123" }
$checks += "password-only change: $($r.success)"
$r = Api "POST" "/api/settings" @{ new_password="admin123" }
$checks += "password restore: $($r.success)"

$page = (Invoke-WebRequest -Uri "$base/settings" -WebSession $s -UseBasicParsing).Content
$checks += "site_name survived: $($page -match 'TestPanelSite')"
$checks += "sub_domain survived: $($page -match 'sub.example.com')"
$checks += "reset cycle weekly selected: $($page -match 'weekly')"

$r = Api "POST" "/api/plans" @{ name="ProPlan"; traffic_gb=300; duration_days=90; price=29.9; device_limit=5; enabled=1 }
$checks += "create plan: $($r.success)"
$r = Api "POST" "/api/users" @{ email="smoke@test.com"; protocol="vless"; plan_id=4; traffic_gb=0; duration_days=0; device_limit=0; enabled=1 }
$checks += "create user: $($r.success)"

$r = Api "GET" "/api/users/1/config"
$checks += "config links: $($r.links.Count) (expect 1)"
$checks += "subscription non-empty: $([bool]$r.subscription)"

$r = Api "PUT" "/api/users/1" @{ email="smoke@test.com"; protocol="vmess"; plan_id=$null; traffic_gb=800; duration_days=0; device_limit=2; password=""; enabled=1 }
$checks += "update user: $($r.success)"
$r = Api "GET" "/api/users/1/config"
$checks += "protocol now vmess: $($r.protocol -eq 'vmess')"
$checks += "link starts vmess://: $($r.links[0].StartsWith('vmess://'))"

$r = Api "POST" "/api/nodes" @{ name="HK-01"; address="hk.example.com"; port=443; network="ws"; security="tls"; sni="hk.example.com"; flow=""; allow_insecure=0; remarks=""; enabled=1 }
$checks += "create node: $($r.success)"
$r = Api "PUT" "/api/nodes/2" @{ name="HK-02"; address="hk.example.com"; port=8443; network="grpc"; security="reality"; sni=""; flow="xtls-rprx-vision"; allow_insecure=1; remarks=""; enabled=1 }
$checks += "update node: $($r.success)"

$r = Api "POST" "/api/traffic"
$checks += "traffic sim: $($r.success)"
$r = Api "POST" "/api/users/1/reset-traffic"
$checks += "reset traffic: $($r.success)"
$r = Api "POST" "/api/users/1/reset-uuid"
$checks += "reset uuid: $($r.success)"

$sub = (Invoke-WebRequest -Uri "$base/api/subscription/1" -WebSession $s -UseBasicParsing).Content
$checks += "subscription content: $([bool]$sub)"
$r = Api "GET" "/api/subscription/all"
$checks += "subscription all: $($r.success) count=$($r.data.PSObject.Properties.Count)"

$pageMarkers = @{
    "/dashboard" = "sidebar"
    "/users" = "smoke@test.com"
    "/plans" = "ProPlan"
    "/nodes" = "HK-02"
    "/subscriptions" = "smoke@test.com"
    "/traffic" = "smoke@test.com"
    "/logs" = "sidebar"
    "/settings" = "TestPanelSite"
}
foreach ($p in @("/dashboard","/users","/plans","/nodes","/subscriptions","/traffic","/logs","/settings")) {
    $pg = (Invoke-WebRequest -Uri "$base$p" -WebSession $s -UseBasicParsing).Content
    $checks += "$p render: $($pg -match $pageMarkers[$p])"
}

$r = Api "DELETE" "/api/users/1"
$checks += "delete user: $($r.success)"
$r = Api "DELETE" "/api/plans/4"
$checks += "delete plan: $($r.success)"
$r = Api "DELETE" "/api/nodes/2"
$checks += "delete node: $($r.success)"

$checks | ForEach-Object { $_ }
$fails = $checks | Where-Object { $_ -match ': (False|True)$' -and $_ -notmatch ': True$' }
if ($fails) { "=== FAILURES ==="; $fails; exit 1 } else { "=== ALL PASSED ===" }
