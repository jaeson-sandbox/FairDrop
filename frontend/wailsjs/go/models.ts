export namespace transfer {
	
	export class PublicError {
	    code: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PublicError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}
	export class ProgressSnapshot {
	    bytesSent: number;
	    totalBytes: number;
	    totalKnown: boolean;
	    percent: number;
	    speedBytesPerSec: number;
	
	    static createFrom(source: any = {}) {
	        return new ProgressSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bytesSent = source["bytesSent"];
	        this.totalBytes = source["totalBytes"];
	        this.totalKnown = source["totalKnown"];
	        this.percent = source["percent"];
	        this.speedBytesPerSec = source["speedBytesPerSec"];
	    }
	}
	export class Event {
	    sessionId: string;
	    seq: number;
	    progress?: ProgressSnapshot;
	    error?: PublicError;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.seq = source["seq"];
	        this.progress = this.convertValues(source["progress"], ProgressSnapshot);
	        this.error = this.convertValues(source["error"], PublicError);
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
	
	

}

