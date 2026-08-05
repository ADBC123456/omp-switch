export namespace config {
	
	export class AppSettings {
	    ompCommand: string;
	    theme: string;
	    darkMode?: boolean;
	    workingDir: string;
	    lastUpdateCheckAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ompCommand = source["ompCommand"];
	        this.theme = source["theme"];
	        this.darkMode = source["darkMode"];
	        this.workingDir = source["workingDir"];
	        this.lastUpdateCheckAt = source["lastUpdateCheckAt"];
	    }
	}
	export class PathView {
	    ompSwitchConfigPath: string;
	    ompModelsPath: string;
	    ompConfigPath: string;
	    ompSessionsDir: string;
	    backupDir: string;
	
	    static createFrom(source: any = {}) {
	        return new PathView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ompSwitchConfigPath = source["ompSwitchConfigPath"];
	        this.ompModelsPath = source["ompModelsPath"];
	        this.ompConfigPath = source["ompConfigPath"];
	        this.ompSessionsDir = source["ompSessionsDir"];
	        this.backupDir = source["backupDir"];
	    }
	}
	export class AppState {
	    version: string;
	    providers: provider.View[];
	    selectedProviderId: string;
	    modelRoles: Record<string, string>;
	    settings: AppSettings;
	    paths: PathView;
	    logs: string[];
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.providers = this.convertValues(source["providers"], provider.View);
	        this.selectedProviderId = source["selectedProviderId"];
	        this.modelRoles = source["modelRoles"];
	        this.settings = this.convertValues(source["settings"], AppSettings);
	        this.paths = this.convertValues(source["paths"], PathView);
	        this.logs = source["logs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RoleUpdate {
	    role: string;
	    selector: string;
	    clear: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RoleUpdate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.selector = source["selector"];
	        this.clear = source["clear"];
	    }
	}

}

export namespace main {
	
	export class ProviderMutationResult {
	    state: config.AppState;
	    finalProviderId: string;
	    adjusted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderMutationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = this.convertValues(source["state"], config.AppState);
	        this.finalProviderId = source["finalProviderId"];
	        this.adjusted = source["adjusted"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace omp {
	
	export class LaunchPreview {
	    executable: string;
	    arguments: string[];
	
	    static createFrom(source: any = {}) {
	        return new LaunchPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.executable = source["executable"];
	        this.arguments = source["arguments"];
	    }
	}

}

export namespace provider {
	
	export class ModelInfo {
	    id: string;
	    name?: string;
	    api?: string;
	    reasoning?: boolean;
	    contextWindow?: number;
	    maxTokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.api = source["api"];
	        this.reasoning = source["reasoning"];
	        this.contextWindow = source["contextWindow"];
	        this.maxTokens = source["maxTokens"];
	    }
	}
	export class DiscoveryResult {
	    models: ModelInfo[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new DiscoveryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.models = this.convertValues(source["models"], ModelInfo);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SaveInput {
	    id: string;
	    baseUrl: string;
	    apiKey: string;
	    api: string;
	    headerMode: string;
	    headers: Record<string, string>;
	    customHeaders: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new SaveInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.api = source["api"];
	        this.headerMode = source["headerMode"];
	        this.headers = source["headers"];
	        this.customHeaders = source["customHeaders"];
	    }
	}
	export class View {
	    id: string;
	    name: string;
	    baseUrl: string;
	    apiKey: string;
	    hasApiKey: boolean;
	    api: string;
	    headerMode: string;
	    headers: Record<string, string>;
	    customHeaders: Record<string, string>;
	    models: ModelInfo[];
	    selectedModelId: string;
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.hasApiKey = source["hasApiKey"];
	        this.api = source["api"];
	        this.headerMode = source["headerMode"];
	        this.headers = source["headers"];
	        this.customHeaders = source["customHeaders"];
	        this.models = this.convertValues(source["models"], ModelInfo);
	        this.selectedModelId = source["selectedModelId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace sessions {
	
	export class Info {
	    id: string;
	    title: string;
	    workingDir: string;
	    model: string;
	    updatedAt: string;
	    sizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.workingDir = source["workingDir"];
	        this.model = source["model"];
	        this.updatedAt = source["updatedAt"];
	        this.sizeBytes = source["sizeBytes"];
	    }
	}

}

export namespace system {
	
	export class EnvCheckResult {
	    name: string;
	    found: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.found = source["found"];
	        this.message = source["message"];
	    }
	}

}

export namespace updater {
	
	export class CheckResult {
	    currentVersion: string;
	    latestVersion: string;
	    hasUpdate: boolean;
	    releaseUrl: string;
	    assetUrl: string;
	    releaseNotes: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.hasUpdate = source["hasUpdate"];
	        this.releaseUrl = source["releaseUrl"];
	        this.assetUrl = source["assetUrl"];
	        this.releaseNotes = source["releaseNotes"];
	    }
	}

}

