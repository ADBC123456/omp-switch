import { escapeHtml } from "./view-utils.js";
import { renderModelRoles } from "./model-roles.js";
import { renderCheckbox } from "./checkbox.js";

function modalFrame({ tone = "", title, description = "", body = "", actions = "", wide = false, variant = "" }) {
  return `<div class="modal-backdrop"><section class="modal-dialog ${wide ? "modal-dialog--wide" : ""} ${tone ? `modal-dialog--${tone}` : ""} ${variant ? `modal-dialog--${variant}` : ""}" role="dialog" aria-modal="true"><header class="modal-header"><h3>${escapeHtml(title)}</h3>${description ? `<p>${escapeHtml(description)}</p>` : ""}</header>${body ? `<div class="modal-body">${body}</div>` : ""}<footer class="modal-footer">${actions}</footer></section></div>`;
}

function resultModal(payload) {
  return modalFrame({ tone: payload.status === "success" ? "success" : "error", title: payload.title, description: payload.message, body: payload.details?.length ? `<div class="result-list">${payload.details.map((item) => `<div class="result-row"><span></span><p>${escapeHtml(item)}</p></div>`).join("")}</div>` : "", actions: '<button class="text-button text-button--accent" data-close-modal>知道了</button>' });
}

function loadingModal(payload) {
  return modalFrame({ title: payload.title, description: payload.message, body: '<div class="loading-line" role="progressbar" aria-label="正在处理"><span></span></div>', actions: payload.requestId ? '<button class="text-button" data-cancel-discovery>取消获取</button>' : "" });
}

function modelParams(model) {
  const parts = [];
  if (model.maxTokens) parts.push(`最大输出 ${model.maxTokens}`);
  if (model.reasoning != null) parts.push(model.reasoning ? "推理 是" : "推理 否");
  if (model.contextWindow) parts.push(`上下文 ${Math.round(model.contextWindow / 1000)}K`);
  return parts;
}


function modelEditor(payload) {
  const model = payload.model ?? {};
  const option = (value, label) => `<option value="${value}" ${model.api === value ? "selected" : ""}>${label}</option>`;
  const reasoning = model.reasoning === true ? "true" : model.reasoning === false ? "false" : "";
  const fetched = modelParams(model);
  const fetchedHint = fetched.length ? `<p class="model-fetched-hint"><span>已从接口获取</span>${fetched.map((part) => `<b>${escapeHtml(part)}</b>`).join("")}</p>` : "";
  return modalFrame({ wide: true, title: payload.originalId ? "编辑模型" : "添加模型", body: `${fetchedHint}<div class="model-editor-grid"><label class="form-field"><span class="form-field__label">模型 ID</span><input name="modelId" value="${escapeHtml(model.id ?? "")}"></label><label class="form-field"><span class="form-field__label">显示名称（可选）</span><input name="modelName" value="${escapeHtml(model.name ?? "")}"></label><label class="form-field"><span class="form-field__label">接口协议</span><select name="modelApi">${option("", "继承 Provider")}${option("openai-completions", "OpenAI Completions")}${option("openai-responses", "OpenAI Responses")}${option("anthropic-messages", "Anthropic Messages")}${option("google-generative-ai", "Google Generative AI")}</select></label><label class="form-field"><span class="form-field__label">推理</span><select name="modelReasoning"><option value="" ${reasoning === "" ? "selected" : ""}>未知</option><option value="true" ${reasoning === "true" ? "selected" : ""}>是</option><option value="false" ${reasoning === "false" ? "selected" : ""}>否</option></select></label><label class="form-field"><span class="form-field__label">上下文窗口（Tokens）</span><input name="modelContextWindow" type="number" min="0" value="${model.contextWindow || ""}" placeholder="未获取"></label><label class="form-field"><span class="form-field__label">最大输出（Tokens）</span><input name="modelMaxTokens" type="number" min="0" value="${model.maxTokens || ""}" placeholder="未获取"></label></div>${payload.error ? `<p class="form-error" role="alert">${escapeHtml(payload.error)}</p>` : ""}`, actions: '<button class="text-button" data-close-modal>取消</button><button class="text-button text-button--accent" data-save-model>保存模型</button>' });
}

function discoveryReview(payload) {
  const query = payload.query ?? "";
  const selected = new Set(payload.selected ?? []);
  const visible = (payload.added ?? []).filter((model) => !query || `${model.id} ${model.name ?? ""}`.toLowerCase().includes(query.toLowerCase()));
  const selectedVisible = visible.filter((model) => selected.has(model.id)).length;
  const section = (title, models, open, selectable = false, editable = false) => {
    const row = (model) => {
      const params = modelParams(model);
      const main = `<span class="discovery-row__main"><strong>${escapeHtml(model.name || model.id)}</strong><small>${escapeHtml(model.id)}</small>${params.length ? `<small class="discovery-row__params">${escapeHtml(params.join(" · "))}</small>` : ""}</span>`;
      const edit = editable ? `<button class="text-button discovery-row__edit" type="button" data-edit-discovered-model="${escapeHtml(model.id)}">编辑</button>` : "";
      if (selectable) {
        return `<div class="discovery-row ${selected.has(model.id) ? "is-selected" : ""}">${renderCheckbox({ checked: selected.has(model.id), dataset: { reviewModel: model.id }, label: `选择 ${model.name || model.id}`, className: "discovery-row__check" })}${main}${edit}</div>`;
      }
      return `<div class="discovery-row">${main}${edit}</div>`;
    };
    const items = models.length ? models.map((model) => row(model)).join("") : '<p class="quiet-empty">无</p>';
    return `<details class="discovery-group" ${open ? "open" : ""}><summary>${title}<span>${models.length}</span></summary><div class="discovery-list">${items}</div></details>`;
  };
  return modalFrame({ wide: true, variant: "pills discovery-review", title: "识别结果", description: `新增 ${payload.added.length} · 已存在 ${payload.existing.length} · 未识别 ${payload.missing.length}`, body: `<form class="discovery-toolbar" data-discovery-search-form><label class="discovery-search"><span class="discovery-search__label">搜索模型</span><span class="discovery-search__control"><input type="search" data-discovery-search value="${escapeHtml(query)}" placeholder="输入 ID 或名称，按 Enter 搜索"><button class="discovery-search__button" type="submit">搜索</button></span></label><button class="text-button" type="button" data-toggle-filtered-review>${selectedVisible === visible.length && visible.length ? "取消选择筛选项" : "选择筛选项"}</button><span class="discovery-count" aria-live="polite">${selectedVisible}/${visible.length} 已选</span></form>${payload.warnings.length ? `<div class="discovery-warnings">${payload.warnings.map((warning) => `<p>${escapeHtml(warning)}</p>`).join("")}</div>` : ""}${section("新增", visible, true, true, true)}${section("已存在", payload.existing, false, false, true)}${section("本次未识别", payload.missing, false)}`, actions: '<button class="text-button" data-close-modal>取消</button><button class="text-button text-button--accent" data-import-models>导入所选模型</button>' });
}

function settingsModal(state) {
  const field = (label, name, value) => `<label class="form-field"><span class="form-field__label">${label}</span><input name="${name}" value="${escapeHtml(value ?? "")}"></label>`;
  const pathField = (label, name, value) => `<label class="form-field"><span class="form-field__label">${label}</span><input name="${name}" value="${escapeHtml(value ?? "")}" placeholder="留空使用默认路径"></label>`;
  const pathInfo = (label, value) => `<div class="settings-path"><span>${label}</span><code>${escapeHtml(value || "—")}</code></div>`;
  const themeOption = (value, label) => `<option value="${value}" ${state.settings.theme === value ? "selected" : ""}>${label}</option>`;
  const launchMode = state.settings.launchMode || "native";
  const launchModeOption = (value, label) => `<option value="${value}" ${launchMode === value ? "selected" : ""}>${label}</option>`;
  const distroValue = state.settings.wslDistro || "";
  const distroOptions = (state.wslDistros || []).map((d) => {
    const id = (d && d.id) || d;
    const label = (d && d.name) || d || "";
    const parts = [];
    if (d && d.version) parts.push(`WSL${d.version}`);
    if (d && d.isDefault) parts.push("默认");
    const suffix = parts.length ? ` (${parts.join(", ")})` : "";
    return `<option value="${escapeHtml(id)}" ${distroValue === id ? "selected" : ""}>${escapeHtml(label)}${suffix}</option>`;
  }).join("");
  const distroSelect = `<label class="form-field"><span class="form-field__label">WSL 发行版</span><div class="settings-inline"><select name="wslDistro"><option value="">使用默认 WSL</option>${distroOptions}</select><button type="button" class="text-button" data-refresh-wsl-distros title="检测并枚举已安装的 WSL 发行版">检测</button></div></label>`;
  const distroInput = `<label class="form-field"><span class="form-field__label">WSL 发行版</span><div class="settings-inline"><input name="wslDistro" value="${escapeHtml(distroValue)}" placeholder="如 Ubuntu"><button type="button" class="text-button" data-refresh-wsl-distros title="检测并枚举已安装的 WSL 发行版">检测</button></div></label>`;
  const distroControl = (state.wslDistros && state.wslDistros.length > 0) ? distroSelect : distroInput;
  const detectLabel = state.settings.launchMode === "wsl" ? "检测 OMP 路径" : "检测本地 OMP 路径";
  const detectButton = `<button type="button" class="text-button text-button--accent" data-detect-wsl-paths>${detectLabel}</button>`;
  return modalFrame({ wide: true, title: "应用设置", body: `<div class="settings-form"><label class="form-field"><span class="form-field__label">界面主题</span><select name="theme">${themeOption("system", "跟随系统")}${themeOption("light", "浅色")}${themeOption("dark", "深色")}</select></label>${field("OMP 命令", "ompCommand", state.settings.ompCommand)}${field("工作目录", "workingDir", state.settings.workingDir)}<label class="form-field"><span class="form-field__label">启动模式</span><select name="launchMode">${launchModeOption("native", "本机 Windows")}${launchModeOption("wsl", "WSL 子系统")}</select></label>${launchMode === "wsl" ? distroControl : ""}${detectButton ? `<div class="settings-detect-row">${detectButton}<span class="settings-detect-status" data-detect-status aria-live="polite"></span></div>` : ""}<div class="settings-paths">${pathInfo("Switch 配置（不可改）", state.paths?.ompSwitchConfigPath)}${pathInfo("备份目录（不可改）", state.paths?.backupDir)}${pathField("OMP Models 路径", "customOmpModelsPath", state.settings.customPaths?.ompModelsPath)}${pathField("OMP Config 路径", "customOmpConfigPath", state.settings.customPaths?.ompConfigPath)}${pathField("OMP 会话目录", "customOmpSessionsDir", state.settings.customPaths?.ompSessionsDir)}</div><div class="settings-update"><span>v${escapeHtml(state.version)}</span><button class="text-button" data-check-update>检查更新</button><span data-update-status aria-live="polite"></span></div></div>`, actions: '<button class="text-button" data-close-modal>取消</button><button class="text-button text-button--accent" data-save-settings>保存设置</button>' });
}

function confirmation(kind, payload) {
  const roles = payload.roles?.length ? ` 将清除角色：${payload.roles.join("、")}。` : "";
  if (kind === "confirm-delete-provider") {
    return modalFrame({ tone: "error", title: "删除 Provider", description: `确定删除 ${payload.name}？${roles}`, actions: '<button class="text-button" data-close-modal>取消</button><button class="text-button text-button--danger" data-confirm-delete-provider>确认删除</button>' });
  }
  const count = payload.modelIDs?.length ?? 0;
  return modalFrame({ tone: "error", title: "删除模型", description: `确定删除 ${count} 个模型？${roles}`, actions: '<button class="text-button" data-close-modal>取消</button><button class="text-button text-button--danger" data-confirm-delete-models>确认删除</button>' });
}
function modelDetailsModal(payload) {
  const model = payload.model ?? {};
  const row = (label, value) => `<div class="settings-path"><span>${escapeHtml(label)}</span><code>${escapeHtml(value || "未配置")}</code></div>`;
  return modalFrame({ title: model.name || model.id || "模型详情", body: `<div class="settings-paths">${row("模型 ID", model.id)}${row("接口协议", model.api)}${row("推理", model.reasoning == null ? "" : model.reasoning ? "是" : "否")}${row("上下文窗口（Tokens）", model.contextWindow ? String(model.contextWindow) : "")}${row("最大输出（Tokens）", model.maxTokens ? String(model.maxTokens) : "")}</div>`, actions: '<button class="text-button text-button--accent" data-close-modal>关闭</button>' });
}


function providerManager(state) {
  const rows = state.providers.length
    ? state.providers.map((provider) => {
        const selected = provider.id === state.selectedProviderId;
        return `<div class="provider-manage-row ${selected ? "is-selected" : ""}"><button class="provider-manage-row__select" data-provider-id="${escapeHtml(provider.id)}"><span><strong>${escapeHtml(provider.name || provider.id)}</strong><small>${escapeHtml(provider.baseUrl || "未配置地址")}</small></span>${selected ? "<em>当前</em>" : "<span>切换</span>"}</button><button class="text-button text-button--danger" data-delete-provider-id="${escapeHtml(provider.id)}" aria-label="删除 ${escapeHtml(provider.name || provider.id)}">删除</button></div>`;
      }).join("")
    : '<p class="quiet-empty">还没有 Provider，请先添加。</p>';
  return modalFrame({ wide: true, title: "Provider 管理", description: "切换当前 Provider，或删除不再使用的 Provider。", body: `<div class="provider-manage-list">${rows}</div>`, actions: '<button class="text-button" data-close-modal>关闭</button><button class="text-button text-button--accent" data-open-add-provider>添加 Provider</button>' });
}

function sessionManager(payload) {
  const sessions = payload.sessions ?? [];
  const pendingId = payload.pendingSessionId ?? "";
  const dateFormat = new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });
  const size = (bytes) => bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`;
  const rows = sessions.map((session) => {
    const pending = pendingId === session.id;
    return `<article class="session-row"><div class="session-row__content"><div class="session-row__heading"><strong title="${escapeHtml(session.title)}">${escapeHtml(session.title)}</strong><time>${escapeHtml(dateFormat.format(new Date(session.updatedAt)))}</time></div><p title="${escapeHtml(session.workingDir)}">${escapeHtml(session.workingDir)}</p><div class="session-row__meta"><span>${escapeHtml(session.model || "未记录模型")}</span><span>${escapeHtml(size(session.sizeBytes || 0))}</span></div></div><div class="session-row__actions"><button class="text-button text-button--accent" data-continue-session="${escapeHtml(session.id)}" ${pendingId ? "disabled" : ""}>${pending ? '<i class="button-spinner"></i>启动中' : "继续会话"}</button><button class="text-button text-button--danger" data-delete-session="${escapeHtml(session.id)}" ${pendingId ? "disabled" : ""}>删除</button></div></article>`;
  }).join("");
  return modalFrame({ wide: true, title: "会话管理", description: sessions.length ? `共 ${sessions.length} 个 OMP 会话，按最近使用时间排序。` : "当前没有可继续的 OMP 会话。", body: `<div class="session-list">${rows || '<p class="quiet-empty session-empty">启动 OMP 并完成一次对话后，会话会显示在这里。</p>'}</div>`, actions: '<button class="text-button text-button--accent" data-close-modal>关闭</button>' });
}

function deleteSessionConfirmation(payload) {
  const session = payload.session ?? {};
  return modalFrame({ tone: "error", title: "删除会话", description: `确定删除“${session.title || "未命名会话"}”？会话记录和附件将永久删除。`, actions: '<button class="text-button" data-return-sessions>取消</button><button class="text-button text-button--danger" data-confirm-delete-session>确认删除</button>' });
}

function skillManager(payload) {
  const skills = payload.skills ?? [];
  const rows = skills.map((skill) => `<article class="skill-row"><div class="skill-row__content"><div class="skill-row__heading"><strong>${escapeHtml(skill.name)}</strong>${skill.locked ? '<span>已登记</span>' : ""}</div><p>${escapeHtml(skill.description || "未提供说明")}</p><code title="${escapeHtml(skill.path)}">${escapeHtml(skill.path)}</code></div><button class="text-button text-button--danger" data-delete-global-skill="${escapeHtml(skill.name)}" aria-label="全局删除 ${escapeHtml(skill.name)}">删除</button></article>`).join("");
  return modalFrame({ wide: true, title: "OMP Skill", description: skills.length ? `全局目录中共 ${skills.length} 个可用 Skill。` : "全局目录中没有可用 Skill。", body: `<div class="skill-root"><span>全局目录</span><code title="${escapeHtml(payload.root || "")}">${escapeHtml(payload.root || "未配置")}</code></div><div class="skill-list">${rows || '<p class="quiet-empty skill-empty">安装全局 Skill 后会显示在这里。</p>'}</div>`, actions: '<button class="text-button text-button--accent" data-close-modal>关闭</button>' });
}

function deleteSkillConfirmation(payload) {
  const skill = payload.skill ?? {};
  return modalFrame({ tone: "error", title: "全局删除 Skill", description: `确定删除“${skill.name || "未命名 Skill"}”？Skill 目录及其全局登记将被永久删除，无法恢复。`, body: `<div class="delete-skill-path"><span>将删除</span><code>${escapeHtml(skill.path || "")}</code></div>`, actions: '<button class="text-button" data-return-global-skills>取消</button><button class="text-button text-button--danger" data-confirm-delete-skill>确认删除</button>' });
}

export function renderModal(state) {
  const modal = state.modal; if (!modal) return "";
  if (modal.kind === "operation-loading") return loadingModal(modal.payload);
  if (modal.kind === "operation-result") return resultModal(modal.payload);
  if (modal.kind === "model-editor") return modelEditor(modal.payload);
  if (modal.kind === "discovery-review") return discoveryReview(modal.payload);
  if (modal.kind === "settings") return settingsModal(state);
  if (modal.kind === "model-details") return modelDetailsModal(modal.payload);
  if (modal.kind === "provider-manager") return providerManager(state);
  if (modal.kind === "session-manager") return sessionManager(modal.payload);
  if (modal.kind === "confirm-delete-session") return deleteSessionConfirmation(modal.payload);
  if (modal.kind === "skill-manager") return skillManager(modal.payload);
  if (modal.kind === "confirm-delete-skill") return deleteSkillConfirmation(modal.payload);
  if (modal.kind === "model-roles") return modalFrame({ wide: true, title: "模型角色", description: "仅更新本面板中明确修改的角色。", body: renderModelRoles(state, modal.payload), actions: '<button class="text-button" data-close-modal>取消</button><button class="text-button text-button--accent" data-save-roles>保存角色</button>' });
  if (modal.kind === "add-provider") return modalFrame({ wide: true, variant: "pills preset-picker", title: "选择配置模板", body: `<div class="preset-list">${state.presets.map((preset) => `<button class="preset-row" data-preset-id="${escapeHtml(preset.id)}"><span><strong>${escapeHtml(preset.label)}</strong><small>${escapeHtml(preset.baseUrl)}</small></span><span aria-hidden="true">→</span></button>`).join("")}</div>`, actions: '<button class="text-button" data-close-modal>取消</button>' });
  if (modal.kind === "confirm-delete-provider" || modal.kind === "confirm-delete-models") return confirmation(modal.kind, modal.payload);
  if (modal.kind === "update-available") return modalFrame({ tone: "success", title: "发现新版本", description: `新版本 v${modal.payload.latestVersion} 已可用。将打开 GitHub Releases 页面下载安装。`, actions: '<button class="text-button" data-skip-update>稍后再说</button><button class="text-button text-button--accent" data-install-update>前往 GitHub 下载</button>' });
  return "";
}
