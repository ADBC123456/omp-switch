import { CancelModelDiscovery, CheckEnvVar, CheckForUpdate, ContinueSession, CreateProvider, DeleteModel, DeleteProvider, DeleteSession, ExecuteLaunchOMP, FetchModels, GetAppState, GetModelDeleteImpact, GetProviderDeleteImpact, ImportDiscoveredModels, InstallUpdate, ListSessions, MarkUpdateChecked, OpenConfigFolder, RestartOMP, SaveModel, SetSelectedModel, SetSelectedProvider, UpdateModelRoles, UpdateProvider, UpdateSettings } from "../../wailsjs/go/main/App";
import { BrowserOpenURL, Quit, WindowMinimise, WindowToggleMaximise, EventsOn } from "../../wailsjs/runtime/runtime";

export class WailsApi {
  onConfigChanged(callback) { return EventsOn("omp:config-changed", callback); }
  onUpdateAvailable(callback) { return EventsOn("omp:update-available", callback); }
  getAppState() { return GetAppState(); }
  createProvider(input) { return CreateProvider(input); }
  updateProvider(id, input) { return UpdateProvider(id, input); }
  deleteProvider(id) { return DeleteProvider(id); }
  getProviderDeleteImpact(id) { return GetProviderDeleteImpact(id); }
  fetchModels(id, requestId) { return FetchModels(id, requestId); }
  cancelModelDiscovery(requestId) { return CancelModelDiscovery(requestId); }
  importDiscoveredModels(id, models) { return ImportDiscoveredModels(id, models); }
  saveModel(providerId, originalId, model) { return SaveModel(providerId, originalId, model); }
  deleteModel(providerId, modelId) { return DeleteModel(providerId, modelId); }
  getModelDeleteImpact(providerId, modelId) { return GetModelDeleteImpact(providerId, modelId); }
  setSelectedProvider(id) { return SetSelectedProvider(id); }
  setSelectedModel(providerId, modelId) { return SetSelectedModel(providerId, modelId); }
  updateModelRoles(updates) { return UpdateModelRoles(updates); }
  executeLaunchOMP(providerId, modelId) { return ExecuteLaunchOMP(providerId, modelId); }
  restartOMP() { return RestartOMP(); }
  listSessions() { return ListSessions(); }
  continueSession(id) { return ContinueSession(id); }
  deleteSession(id) { return DeleteSession(id); }
  updateSettings(settings) { return UpdateSettings(settings); }
  checkForUpdate() { return CheckForUpdate(); }
  markUpdateChecked() { return MarkUpdateChecked(); }
  installUpdate() { return InstallUpdate(); }
  checkEnvVar(name) { return CheckEnvVar(name); }
  openConfigFolder() { return OpenConfigFolder(); }
  openExternalURL(url) { BrowserOpenURL(url); }
  minimiseWindow() { WindowMinimise(); }
  closeWindow() { Quit(); }
  toggleMaximiseWindow() { WindowToggleMaximise(); }
}
