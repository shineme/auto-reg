export namespace main {
	
	export class Config {
	    panelBase: string;
	    panelKey: string;
	    mailMode: string;
	    mailDomain: string;
	    mailPrefix: string;
	    mailbox: string;
	    imapHost: string;
	    imapPort: number;
	    imapUser: string;
	    imapPassword: string;
	    hotmailRaw: string;
	    hotmailPool: string;
	    hotmailPreflightWorkers: number;
	    proxyUrl: string;
	    proxyDirectHosts: string[];
	    count: number;
	    chromePath: string;
	    outputDir: string;
	    scriptsDir: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.panelBase = source["panelBase"];
	        this.panelKey = source["panelKey"];
	        this.mailMode = source["mailMode"];
	        this.mailDomain = source["mailDomain"];
	        this.mailPrefix = source["mailPrefix"];
	        this.mailbox = source["mailbox"];
	        this.imapHost = source["imapHost"];
	        this.imapPort = source["imapPort"];
	        this.imapUser = source["imapUser"];
	        this.imapPassword = source["imapPassword"];
	        this.hotmailRaw = source["hotmailRaw"];
	        this.hotmailPool = source["hotmailPool"];
	        this.hotmailPreflightWorkers = source["hotmailPreflightWorkers"];
	        this.proxyUrl = source["proxyUrl"];
	        this.proxyDirectHosts = source["proxyDirectHosts"];
	        this.count = source["count"];
	        this.chromePath = source["chromePath"];
	        this.outputDir = source["outputDir"];
	        this.scriptsDir = source["scriptsDir"];
	    }
	}
	export class HotmailImportResult {
	    ok: boolean;
	    filePath: string;
	    imported: number;
	    updated: number;
	    total: number;
	    available: number;
	    used: number;
	    errors: any[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new HotmailImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.filePath = source["filePath"];
	        this.imported = source["imported"];
	        this.updated = source["updated"];
	        this.total = source["total"];
	        this.available = source["available"];
	        this.used = source["used"];
	        this.errors = source["errors"];
	        this.message = source["message"];
	    }
	}
	export class HotmailPoolCleanupResult {
	    ok: boolean;
	    filePath: string;
	    removed: number;
	    removedEmails: string[];
	    before: number;
	    total: number;
	    available: number;
	    reserved: number;
	    used: number;
	    error: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new HotmailPoolCleanupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.filePath = source["filePath"];
	        this.removed = source["removed"];
	        this.removedEmails = source["removedEmails"];
	        this.before = source["before"];
	        this.total = source["total"];
	        this.available = source["available"];
	        this.reserved = source["reserved"];
	        this.used = source["used"];
	        this.error = source["error"];
	        this.message = source["message"];
	    }
	}
	export class HotmailPoolStatus {
	    ok: boolean;
	    filePath: string;
	    total: number;
	    available: number;
	    reserved: number;
	    used: number;
	    error: number;
	    updatedAt: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new HotmailPoolStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.filePath = source["filePath"];
	        this.total = source["total"];
	        this.available = source["available"];
	        this.reserved = source["reserved"];
	        this.used = source["used"];
	        this.error = source["error"];
	        this.updatedAt = source["updatedAt"];
	        this.message = source["message"];
	    }
	}
	export class HotmailPreflightResult {
	    ok: boolean;
	    filePath: string;
	    releasedReserved: number;
	    recycledSoftErrors: number;
	    checked: number;
	    workers: number;
	    okCount: number;
	    errorCount: number;
	    available: number;
	    reserved: number;
	    used: number;
	    error: number;
	    total: number;
	    results: any[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new HotmailPreflightResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.filePath = source["filePath"];
	        this.releasedReserved = source["releasedReserved"];
	        this.recycledSoftErrors = source["recycledSoftErrors"];
	        this.checked = source["checked"];
	        this.workers = source["workers"];
	        this.okCount = source["okCount"];
	        this.errorCount = source["errorCount"];
	        this.available = source["available"];
	        this.reserved = source["reserved"];
	        this.used = source["used"];
	        this.error = source["error"];
	        this.total = source["total"];
	        this.results = source["results"];
	        this.message = source["message"];
	    }
	}
	export class HotmailTestResult {
	    ok: boolean;
	    email: string;
	    scope: string;
	    messageCount: number;
	    latestSubject: string;
	    checkedAt: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new HotmailTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.email = source["email"];
	        this.scope = source["scope"];
	        this.messageCount = source["messageCount"];
	        this.latestSubject = source["latestSubject"];
	        this.checkedAt = source["checkedAt"];
	        this.message = source["message"];
	    }
	}
	export class PanelCheckResult {
	    ok: boolean;
	    panelBase: string;
	    cookies: string[];
	    providerKeys: string[];
	    hasProvidersArray: boolean;
	    checkedAt: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PanelCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.panelBase = source["panelBase"];
	        this.cookies = source["cookies"];
	        this.providerKeys = source["providerKeys"];
	        this.hasProvidersArray = source["hasProvidersArray"];
	        this.checkedAt = source["checkedAt"];
	        this.message = source["message"];
	    }
	}
	export class Result {
	    ok: boolean;
	    email: string;
	    user: string;
	    userCode: string;
	    signInId: string;
	    flowStatus: string;
	    signInState: string;
	    error: string;
	    runDir: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.email = source["email"];
	        this.user = source["user"];
	        this.userCode = source["userCode"];
	        this.signInId = source["signInId"];
	        this.flowStatus = source["flowStatus"];
	        this.signInState = source["signInState"];
	        this.error = source["error"];
	        this.runDir = source["runDir"];
	    }
	}
	export class RunStatus {
	    running: boolean;
	    phase: string;
	    message: string;
	    count: number;
	    done: number;
	    ok: number;
	    failed: number;
	    attempts: number;
	    current: number;
	    restartCount: number;
	    batchId: string;
	    supervisorPid: number;
	
	    static createFrom(source: any = {}) {
	        return new RunStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.phase = source["phase"];
	        this.message = source["message"];
	        this.count = source["count"];
	        this.done = source["done"];
	        this.ok = source["ok"];
	        this.failed = source["failed"];
	        this.attempts = source["attempts"];
	        this.current = source["current"];
	        this.restartCount = source["restartCount"];
	        this.batchId = source["batchId"];
	        this.supervisorPid = source["supervisorPid"];
	    }
	}
	export class RunEvent {
	    type: string;
	    level: string;
	    message: string;
	    status?: RunStatus;
	    result?: Result;
	    at: string;
	
	    static createFrom(source: any = {}) {
	        return new RunEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.level = source["level"];
	        this.message = source["message"];
	        this.status = this.convertValues(source["status"], RunStatus);
	        this.result = this.convertValues(source["result"], Result);
	        this.at = source["at"];
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

