export namespace transfer {
	
	export class ConnectionOption {
	    id: string;
	    name: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.address = source["address"];
	    }
	}
	export class ConnectionOptions {
	    sessionId: string;
	    currentAddress: string;
	    choices: ConnectionOption[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectionOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.currentAddress = source["currentAddress"];
	        this.choices = this.convertValues(source["choices"], ConnectionOption);
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
	export class ReceiveMetadata {
	    sessionId: string;
	    destination: string;
	    url: string;
	    qrBase64: string;
	    warnings: Warning[];
	
	    static createFrom(source: any = {}) {
	        return new ReceiveMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.destination = source["destination"];
	        this.url = source["url"];
	        this.qrBase64 = source["qrBase64"];
	        this.warnings = this.convertValues(source["warnings"], Warning);
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
	export class Warning {
	    code: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Warning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}
	export class FileMetadata {
	    sessionId: string;
	    name: string;
	    size: number;
	    isDir: boolean;
	    isCollection: boolean;
	    itemCount: number;
	    url: string;
	    qrBase64: string;
	    warnings: Warning[];
	
	    static createFrom(source: any = {}) {
	        return new FileMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.isDir = source["isDir"];
	        this.isCollection = source["isCollection"];
	        this.itemCount = source["itemCount"];
	        this.url = source["url"];
	        this.qrBase64 = source["qrBase64"];
	        this.warnings = this.convertValues(source["warnings"], Warning);
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
	export class ConnectionRefresh {
	    mode: string;
	    send?: FileMetadata;
	    receive?: ReceiveMetadata;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionRefresh(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.send = this.convertValues(source["send"], FileMetadata);
	        this.receive = this.convertValues(source["receive"], ReceiveMetadata);
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

