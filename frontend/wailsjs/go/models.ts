export namespace main {
	
	export class CheckResult {
	    ip: nettest.IPInfo;
	    sites: nettest.Site[];
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = this.convertValues(source["ip"], nettest.IPInfo);
	        this.sites = this.convertValues(source["sites"], nettest.Site);
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
	export class ScanResult {
	    ip: string;
	    port: number;
	    ms: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.port = source["port"];
	        this.ms = source["ms"];
	    }
	}
	export class Status {
	    running: boolean;
	    state: string;
	    mode: string;
	    since: string;
	    admin: boolean;
	    os: string;
	    bins_ok: boolean;
	    missing: string[];
	    error: string;
	    blocking: boolean;
	    pending: boolean;
	    server: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.state = source["state"];
	        this.mode = source["mode"];
	        this.since = source["since"];
	        this.admin = source["admin"];
	        this.os = source["os"];
	        this.bins_ok = source["bins_ok"];
	        this.missing = source["missing"];
	        this.error = source["error"];
	        this.blocking = source["blocking"];
	        this.pending = source["pending"];
	        this.server = source["server"];
	        this.warnings = source["warnings"];
	    }
	}

}

export namespace nettest {
	
	export class IPInfo {
	    ip: string;
	    country: string;
	    colo: string;
	    warp: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new IPInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.country = source["country"];
	        this.colo = source["colo"];
	        this.warp = source["warp"];
	        this.error = source["error"];
	    }
	}
	export class Site {
	    name: string;
	    url: string;
	    ok: boolean;
	    status: number;
	    ms: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Site(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.ms = source["ms"];
	        this.error = source["error"];
	    }
	}

}

export namespace settings {
	
	export class Routing {
	    direct_vn: boolean;
	    direct_domains: string[];
	    proxy_domains: string[];
	    direct_processes: string[];
	
	    static createFrom(source: any = {}) {
	        return new Routing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.direct_vn = source["direct_vn"];
	        this.direct_domains = source["direct_domains"];
	        this.proxy_domains = source["proxy_domains"];
	        this.direct_processes = source["direct_processes"];
	    }
	}
	export class Server {
	    id: string;
	    name: string;
	    link: string;
	    sub_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.link = source["link"];
	        this.sub_id = source["sub_id"];
	    }
	}
	export class Warp {
	    private_key: string;
	    peer_public_key: string;
	    endpoint: string;
	    endpoint_port: number;
	    addresses: string[];
	    reserved: number[];
	    device_id?: string;
	    token?: string;
	    license?: string;
	    account_type?: string;
	
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
	        this.device_id = source["device_id"];
	        this.token = source["token"];
	        this.license = source["license"];
	        this.account_type = source["account_type"];
	    }
	}
	export class Subscription {
	    id: string;
	    name: string;
	    url: string;
	    updated?: string;
	
	    static createFrom(source: any = {}) {
	        return new Subscription(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.updated = source["updated"];
	    }
	}
	export class Settings {
	    mode: string;
	    doh_server: string;
	    bypass_lan: boolean;
	    servers: Server[];
	    subscriptions: Subscription[];
	    selected_server: string;
	    custom_link?: string;
	    warp?: Warp;
	    routing: Routing;
	    kill_switch: boolean;
	    auto_reconnect: boolean;
	    auto_connect: boolean;
	    start_with_windows: boolean;
	    start_minimized: boolean;
	    close_to_tray: boolean;
	    notifications: boolean;
	    interrupt_on_switch: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.doh_server = source["doh_server"];
	        this.bypass_lan = source["bypass_lan"];
	        this.servers = this.convertValues(source["servers"], Server);
	        this.subscriptions = this.convertValues(source["subscriptions"], Subscription);
	        this.selected_server = source["selected_server"];
	        this.custom_link = source["custom_link"];
	        this.warp = this.convertValues(source["warp"], Warp);
	        this.routing = this.convertValues(source["routing"], Routing);
	        this.kill_switch = source["kill_switch"];
	        this.auto_reconnect = source["auto_reconnect"];
	        this.auto_connect = source["auto_connect"];
	        this.start_with_windows = source["start_with_windows"];
	        this.start_minimized = source["start_minimized"];
	        this.close_to_tray = source["close_to_tray"];
	        this.notifications = source["notifications"];
	        this.interrupt_on_switch = source["interrupt_on_switch"];
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

