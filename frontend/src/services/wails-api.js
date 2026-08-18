import { CancelModelDiscovery, CheckEnvVar, CheckForUpdate, ContinueSession, CreateProvider, DeleteGlobalSkill, DeleteModels, DeleteProvider, DeleteSession, ExecuteLaunchOMP, FetchModels, GetAppState, GetModelsDeleteImpact, GetProviderDeleteImpact, ImportDiscoveredModels, InstallUpdate, ListGlobalSkills, ListSessions, MarkUpdateChecked, OpenConfigFolder, SaveModel, SetSelectedModel, SetSelectedProvider, TestModel, UpdateModelRoles, UpdateProvider, UpdateSettings, ListWSLDistros, ResolveWSLPaths, ResolveNativePaths, } from "../../wailsjs/go/main/App";
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
  deleteModels(providerId, modelIds) { return DeleteModels(providerId, modelIds); }
  getModelsDeleteImpact(providerId, modelIds) { return GetModelsDeleteImpact(providerId, modelIds); }
  setSelectedProvider(id) { return SetSelectedProvider(id); }
  setSelectedModel(providerId, modelId) { return SetSelectedModel(providerId, modelId); }
  updateModelRoles(updates) { return UpdateModelRoles(updates); }
  executeLaunchOMP(providerId, modelId) { return ExecuteLaunchOMP(providerId, modelId); }
  testModel(providerId, modelId) { return TestModel(providerId, modelId); }
  listSessions() { return ListSessions(); }
  continueSession(id) { return ContinueSession(id); }
  deleteSession(id) { return DeleteSession(id); }
  listGlobalSkills() { return ListGlobalSkills(); }
  deleteGlobalSkill(name) { return DeleteGlobalSkill(name); }
  updateSettings(settings) { return UpdateSettings(settings); }
  listWSLDistros() { return ListWSLDistros(); }
  resolveWSLPaths(distro) { return ResolveWSLPaths(distro); }
  resolveNativePaths() { return ResolveNativePaths(); }
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
