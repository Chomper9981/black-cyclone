export namespace main {
	
	export class Status {
	    running: boolean;
	    mode: string;
	    since: string;
	    admin: boolean;
	    bins_ok: boolean;
	    missing: string[];
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.mode = source["mode"];
	        this.since = source["since"];
	        this.admin = source["admin"];
	        this.bins_ok = source["bins_ok"];
	        this.missing = source["missing"];
	        this.error = source["error"];
	    }
	}

}

export namespace settings {
	
	export class Warp {
	    private_key: string;
	    peer_public_key: string;
	    endpoint: string;
	    endpoint_port: number;
	    addresses: string[];
	    reserved: number[];
	
	    static createFrom(source: any = {}) {
	        return new Warp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.private_key = source["private_key"];
	        this.peer_public_key = source["peer_public_key"];
	        this.endpoint = source["endpoint"];
	        this.endpoint_port = source["endpoint_port"];
	        this.addresses = source["addresses"];
	        this.reserved = source["reserved"];
	    }
	}
	export class Settings {
	    mode: string;
	    doh_server: string;
	    use_dpi: boolean;
	    custom_link: string;
	    bypass_lan: boolean;
	    warp?: Warp;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.doh_server = source["doh_server"];
	        this.use_dpi = source["use_dpi"];
	        this.custom_link = source["custom_link"];
	        this.bypass_lan = source["bypass_lan"];
	        this.warp = this.convertValues(source["warp"], Warp);
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

