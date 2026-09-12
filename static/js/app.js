/* ================= 通用 ================= */
function toast(msg, type = "success") {
  const el = document.getElementById("toast");
  el.textContent = msg;
  el.className = "toast show " + type;
  clearTimeout(el._t);
  el._t = setTimeout(() => (el.className = "toast"), 3000);
}

async function api(url, method = "GET", data = null) {
  const opts = { method, headers: {} };
  if (data) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(data);
  }
  const res = await fetch(url, opts);
  if (res.status === 401) {
    window.location.href = "/login";
    throw new Error("未登录");
  }
  const json = await res.json().catch(() => ({}));
  if (!json.success) throw new Error(json.msg || "操作失败");
  return json;
}

function openModal(id) {
  document.getElementById(id).classList.add("open");
}
function closeModal(id) {
  document.getElementById(id).classList.remove("open");
}
document.addEventListener("click", (e) => {
  if (e.target.classList.contains("modal-overlay")) e.target.classList.remove("open");
});

function copyText(inputId) {
  const el = document.getElementById(inputId);
  navigator.clipboard.writeText(el.textContent).then(
    () => toast("已复制到剪贴板"),
    () => toast("复制失败，请手动复制", "error")
  );
}

/* ================= 用户管理 ================= */
function openUserModal(id) {
  const modal = document.getElementById("userModal");
  document.getElementById("userId").value = "";
  document.getElementById("userEmail").value = "";
  document.getElementById("userProtocol").value = "vless";
  document.getElementById("userPlan").value = "";
  document.getElementById("userTraffic").value = 50;
  document.getElementById("userDuration").value = 30;
  document.getElementById("userDevice").value = 0;
  document.getElementById("userPassword").value = "";
  document.getElementById("userEnabled").checked = true;
  document.getElementById("userModalTitle").textContent = "添加用户";
  if (id) {
    const tr = document.querySelector(`tr[data-id="${id}"]`);
    if (tr) {
      const d = tr.dataset;
      document.getElementById("userId").value = id;
      document.getElementById("userModalTitle").textContent = "编辑用户 #" + id;
      const emailInput = document.getElementById("userEmail");
      emailInput.value = tr.querySelector("td:nth-child(2)").textContent.trim();
      document.getElementById("userProtocol").value = d.protocol;
      document.getElementById("userPlan").value = d.planId;
      document.getElementById("userTraffic").value = d.trafficGb;
      document.getElementById("userDuration").value = "";
      document.getElementById("userDevice").value = d.deviceLimit;
      document.getElementById("userEnabled").checked = d.enabled === "1";
      if (d.expireAt) {
        const dt = new Date(d.expireAt.replace(" ", "T"));
        const days = Math.ceil((dt - new Date()) / 86400000);
        document.getElementById("userDuration").value = days > 0 ? days : 0;
      } else {
        document.getElementById("userDuration").value = 0;
      }
    }
  }
  openModal("userModal");
}

async function saveUser() {
  const id = document.getElementById("userId").value;
  const email = document.getElementById("userEmail").value.trim();
  if (!email) return toast("请输入邮箱", "error");
  const data = {
    email,
    protocol: document.getElementById("userProtocol").value,
    plan_id: parseInt(document.getElementById("userPlan").value) || null,
    traffic_gb: parseInt(document.getElementById("userTraffic").value) || 0,
    duration_days: parseInt(document.getElementById("userDuration").value) || 0,
    device_limit: parseInt(document.getElementById("userDevice").value) || 0,
    password: document.getElementById("userPassword").value,
    enabled: document.getElementById("userEnabled").checked ? 1 : 0,
  };
  try {
    if (id) {
      await api(`/api/users/${id}`, "PUT", data);
    } else {
      await api("/api/users", "POST", data);
    }
    toast("保存成功");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

async function deleteUser(id) {
  if (!confirm("确定删除该用户？其流量记录也将被删除。")) return;
  try {
    await api(`/api/users/${id}`, "DELETE");
    toast("已删除");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

async function resetTraffic(id) {
  if (!confirm("确定重置该用户已用流量？")) return;
  try {
    await api(`/api/users/${id}/reset-traffic`, "POST");
    toast("流量已重置");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

async function resetUuid(id) {
  if (!confirm("重置 UUID 后用户需重新导入配置，确定继续？")) return;
  try {
    await api(`/api/users/${id}/reset-uuid`, "POST");
    toast("UUID 已重置");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

function filterUsers() {
  const kw = document.getElementById("userSearch").value.toLowerCase();
  document.querySelectorAll("#usersTable tbody tr").forEach((tr) => {
    tr.style.display = (tr.dataset.email || "").toLowerCase().includes(kw) ? "" : "none";
  });
}

/* ================= 用户配置 ================= */
let currentConfig = null;

async function openConfig(id) {
  try {
    const res = await api(`/api/users/${id}/config`);
    currentConfig = res;
    document.getElementById("cfgUuid").textContent = res.uuid;
    document.getElementById("cfgPassword").textContent = res.password;
    const protoEl = document.getElementById("cfgProtocol");
    protoEl.textContent = res.protocol;
    protoEl.className = "badge badge-" + res.protocol;
    document.getElementById("linkCount").textContent = res.links.length;

    const list = document.getElementById("configLinks");
    list.innerHTML = "";
    res.links.forEach((link) => {
      const div = document.createElement("div");
      div.className = "link-item";
      div.innerHTML = `<code>${escapeHtml(link)}</code>
        <button class="btn-mini" onclick="copyRaw(this)">复制</button>`;
      list.appendChild(div);
    });

    const qr = document.getElementById("qrCode");
    if (res.links.length) {
      drawQR(qr, res.links[0]);
    } else {
      qr.textContent = "无可用节点";
      qr.style.color = "#666";
    }
    openModal("configModal");
  } catch (e) {
    toast(e.message, "error");
  }
}

function copyRaw(btn) {
  const code = btn.parentElement.querySelector("code").textContent;
  navigator.clipboard.writeText(code).then(
    () => toast("已复制"),
    () => toast("复制失败", "error")
  );
}

function copySub() {
  if (currentConfig && currentConfig.subscription) {
    navigator.clipboard.writeText(currentConfig.subscription).then(
      () => toast("订阅内容已复制"),
      () => toast("复制失败", "error")
    );
  }
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

/* 轻量 QR 码生成 (二维码矩阵 + SVG 渲染) */
function drawQR(el, text) {
  try {
    const qr = qrcode(0, "M");
    qr.addData(text);
    qr.make();
    const count = qr.getModuleCount();
    let cells = "";
    for (let r = 0; r < count; r++) {
      for (let c = 0; c < count; c++) {
        if (qr.isDark(r, c)) cells += `<rect x="${c}" y="${r}" width="1" height="1"/>`;
      }
    }
    el.innerHTML =
      `<svg viewBox="0 0 ${count} ${count}" xmlns="http://www.w3.org/2000/svg" ` +
      `shape-rendering="crispEdges" style="padding:6px;background:#fff">` +
      `<g fill="#111">${cells}</g></svg>`;
    el.style.color = "";
  } catch (e) {
    el.textContent = "二维码生成失败";
  }
}

/* ================= 套餐管理 ================= */
function openPlanModal(id) {
  document.getElementById("planId").value = "";
  document.getElementById("planName").value = "";
  document.getElementById("planTraffic").value = 50;
  document.getElementById("planDuration").value = 30;
  document.getElementById("planPrice").value = 0;
  document.getElementById("planDevice").value = 0;
  document.getElementById("planEnabled").checked = true;
  document.getElementById("planModalTitle").textContent = "添加套餐";
  if (id) {
    const tr = document.querySelector(`#planModal ~ * tr[data-id="${id}"]`);
    const row = document.querySelector(`.panel table tr[data-id="${id}"]`);
    if (row) {
      const d = row.dataset;
      document.getElementById("planId").value = id;
      document.getElementById("planModalTitle").textContent = "编辑套餐 #" + id;
      document.getElementById("planName").value = d.name;
      document.getElementById("planTraffic").value = d.trafficGb;
      document.getElementById("planDuration").value = d.durationDays;
      document.getElementById("planPrice").value = d.price;
      document.getElementById("planDevice").value = d.deviceLimit;
      document.getElementById("planEnabled").checked = d.enabled === "1";
    }
  }
  openModal("planModal");
}

async function savePlan() {
  const id = document.getElementById("planId").value;
  const name = document.getElementById("planName").value.trim();
  if (!name) return toast("请输入套餐名称", "error");
  const data = {
    name,
    traffic_gb: parseInt(document.getElementById("planTraffic").value) || 0,
    duration_days: parseInt(document.getElementById("planDuration").value) || 0,
    price: parseFloat(document.getElementById("planPrice").value) || 0,
    device_limit: parseInt(document.getElementById("planDevice").value) || 0,
    enabled: document.getElementById("planEnabled").checked ? 1 : 0,
  };
  try {
    if (id) {
      await api(`/api/plans/${id}`, "PUT", data);
    } else {
      await api("/api/plans", "POST", data);
    }
    toast("保存成功");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

async function deletePlan(id) {
  if (!confirm("确定删除该套餐？绑定此套餐的用户不受影响。")) return;
  try {
    await api(`/api/plans/${id}`, "DELETE");
    toast("已删除");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

/* ================= 节点管理 ================= */
function openNodeModal(id) {
  document.getElementById("nodeId").value = "";
  document.getElementById("nodeName").value = "";
  document.getElementById("nodeAddress").value = "";
  document.getElementById("nodePort").value = 443;
  document.getElementById("nodeSni").value = "";
  document.getElementById("nodeNetwork").value = "ws";
  document.getElementById("nodeSecurity").value = "tls";
  document.getElementById("nodeFlow").value = "";
  document.getElementById("nodeRemarks").value = "";
  document.getElementById("nodeAllowInsecure").checked = false;
  document.getElementById("nodeEnabled").checked = true;
  document.getElementById("nodeModalTitle").textContent = "添加节点";
  if (id) {
    const row = document.querySelector(`.panel table tr[data-id="${id}"]`);
    if (row) {
      const d = row.dataset;
      document.getElementById("nodeId").value = id;
      document.getElementById("nodeModalTitle").textContent = "编辑节点 #" + id;
      document.getElementById("nodeName").value = d.name;
      document.getElementById("nodeAddress").value = d.address;
      document.getElementById("nodePort").value = d.port;
      document.getElementById("nodeSni").value = d.sni;
      document.getElementById("nodeNetwork").value = d.network;
      document.getElementById("nodeSecurity").value = d.security;
      document.getElementById("nodeFlow").value = d.flow;
      document.getElementById("nodeRemarks").value = d.remarks;
      document.getElementById("nodeAllowInsecure").checked = d.allowInsecure === "1";
      document.getElementById("nodeEnabled").checked = d.enabled === "1";
    }
  }
  openModal("nodeModal");
}

async function saveNode() {
  const id = document.getElementById("nodeId").value;
  const name = document.getElementById("nodeName").value.trim();
  const address = document.getElementById("nodeAddress").value.trim();
  if (!name || !address) return toast("请填写节点名称和地址", "error");
  const data = {
    name,
    address,
    port: parseInt(document.getElementById("nodePort").value) || 443,
    sni: document.getElementById("nodeSni").value.trim(),
    network: document.getElementById("nodeNetwork").value,
    security: document.getElementById("nodeSecurity").value,
    flow: document.getElementById("nodeFlow").value,
    remarks: document.getElementById("nodeRemarks").value.trim(),
    allow_insecure: document.getElementById("nodeAllowInsecure").checked ? 1 : 0,
    enabled: document.getElementById("nodeEnabled").checked ? 1 : 0,
  };
  try {
    if (id) {
      await api(`/api/nodes/${id}`, "PUT", data);
    } else {
      await api("/api/nodes", "POST", data);
    }
    toast("保存成功");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

async function deleteNode(id) {
  if (!confirm("确定删除该节点？")) return;
  try {
    await api(`/api/nodes/${id}`, "DELETE");
    toast("已删除");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

/* ================= 订阅 ================= */
async function copySubById(id) {
  try {
    const res = await fetch(`/api/subscription/${id}`);
    const text = await res.text();
    navigator.clipboard.writeText(text).then(
      () => toast("订阅内容已复制"),
      () => toast("复制失败", "error")
    );
  } catch (e) {
    toast("获取订阅失败", "error");
  }
}

async function downloadSub(id) {
  try {
    const res = await fetch(`/api/subscription/${id}`);
    const text = await res.text();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
    a.download = `subscription_${id}.txt`;
    a.click();
    URL.revokeObjectURL(a.href);
  } catch (e) {
    toast("下载失败", "error");
  }
}

async function copyAllSubs() {
  try {
    const res = await api("/api/subscription/all");
    navigator.clipboard.writeText(JSON.stringify(res.data, null, 2)).then(
      () => toast("全部订阅已复制"),
      () => toast("复制失败", "error")
    );
  } catch (e) {
    toast(e.message, "error");
  }
}

async function exportAllSubs() {
  try {
    const res = await api("/api/subscription/all");
    const a = document.createElement("a");
    a.href = URL.createObjectURL(
      new Blob([JSON.stringify(res.data, null, 2)], { type: "application/json" })
    );
    a.download = "subscriptions.json";
    a.click();
    URL.revokeObjectURL(a.href);
  } catch (e) {
    toast(e.message, "error");
  }
}

/* ================= 流量 ================= */
async function recordTraffic() {
  try {
    const res = await api("/api/traffic", "POST");
    toast(res.msg);
    setTimeout(() => location.reload(), 800);
  } catch (e) {
    toast(e.message, "error");
  }
}

/* ================= 日志 ================= */
async function clearLogs() {
  if (!confirm("确定清空全部操作日志？")) return;
  try {
    await api("/api/logs/clear", "POST");
    toast("日志已清空");
    setTimeout(() => location.reload(), 600);
  } catch (e) {
    toast(e.message, "error");
  }
}

/* ================= 设置 ================= */
async function saveSettings() {
  try {
    await api("/api/settings", "POST", {
      site_name: document.getElementById("siteName").value.trim(),
      sub_domain: document.getElementById("subDomain").value.trim(),
      panel_port: document.getElementById("panelPort").value.trim(),
      traffic_reset_cycle: document.getElementById("resetCycle").value,
    });
    toast("设置已保存" + (document.getElementById("panelPort").value.trim() ? " (端口重启后生效)" : ""));
    setTimeout(() => location.reload(), 800);
  } catch (e) {
    toast(e.message, "error");
  }
}

async function changePassword() {
  const p1 = document.getElementById("newPassword").value;
  const p2 = document.getElementById("confirmPassword").value;
  if (p1 !== p2) return toast("两次密码不一致", "error");
  try {
    await api("/api/settings", "POST", { new_password: p1 });
    toast("密码修改成功");
    document.getElementById("newPassword").value = "";
    document.getElementById("confirmPassword").value = "";
  } catch (e) {
    toast(e.message, "error");
  }
}

async function saveTlsSettings() {
  try {
    const res = await api("/api/settings", "POST", {
      tls_enabled: document.getElementById("tlsEnabled").checked ? "1" : "0",
      tls_cert: document.getElementById("tlsCert").value.trim(),
      tls_key: document.getElementById("tlsKey").value.trim(),
      tls_redirect_http: document.getElementById("tlsRedirect").checked ? "1" : "0",
    });
    toast(res.msg + ", 重启面板后生效");
  } catch (e) {
    toast(e.message, "error");
  }
}

async function generateCert() {
  const hosts = document.getElementById("certHosts").value.trim();
  try {
    const res = await api("/api/cert/generate", "POST", { hosts });
    document.getElementById("tlsCert").value = res.cert;
    document.getElementById("tlsKey").value = res.key;
    toast("自签名证书已生成 (10 年有效期)");
  } catch (e) {
    toast(e.message, "error");
  }
}
