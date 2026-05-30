export namespace manager {
	
	export class ActionOptions {
	    version?: string;
	    dry_run?: boolean;
	    verbose?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ActionOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.dry_run = source["dry_run"];
	        this.verbose = source["verbose"];
	    }
	}
	export class ActionResult {
	    success?: boolean;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	    }
	}
	export class Capabilities {
	    list_installed?: boolean;
	    list_outdated?: boolean;
	    get_package_info?: boolean;
	    search?: boolean;
	    install?: boolean;
	    uninstall?: boolean;
	    update?: boolean;
	    list_versions?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Capabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list_installed = source["list_installed"];
	        this.list_outdated = source["list_outdated"];
	        this.get_package_info = source["get_package_info"];
	        this.search = source["search"];
	        this.install = source["install"];
	        this.uninstall = source["uninstall"];
	        this.update = source["update"];
	        this.list_versions = source["list_versions"];
	    }
	}
	export class Info {
	    id?: string;
	    exec_name?: string;
	    name?: string;
	    enabled?: boolean;
	    exec_path?: string;
	    capabilities?: Capabilities;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.exec_name = source["exec_name"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.exec_path = source["exec_path"];
	        this.capabilities = this.convertValues(source["capabilities"], Capabilities);
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
	export class Package {
	    name?: string;
	    fullname?: string;
	    version?: string;
	    latest_version?: string;
	    manager?: string;
	    installed: boolean;
	    outdated: boolean;
	    is_gui: boolean;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new Package(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fullname = source["fullname"];
	        this.version = source["version"];
	        this.latest_version = source["latest_version"];
	        this.manager = source["manager"];
	        this.installed = source["installed"];
	        this.outdated = source["outdated"];
	        this.is_gui = source["is_gui"];
	        this.description = source["description"];
	    }
	}

}

