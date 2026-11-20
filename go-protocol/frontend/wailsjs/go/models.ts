export namespace main {
	
	export class AccountResult {
	    attempt: number;
	    email: string;
	    providerName: string;
	    userCode: string;
	    signInId: string;
	    ok: boolean;
	    error?: string;
	    signin?: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attempt = source["attempt"];
	        this.email = source["email"];
	        this.providerName = source["providerName"];
	        this.userCode = source["userCode"];
	        this.signInId = source["signInId"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.signin = source["signin"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class BatchSummary {
	    target: number;
	    succeeded: number;
	    failedAttempts: number;
	    currentStage: string;
	    currentEmail: string;
	    running: boolean;
	    updatedAt: string;
	    results: AccountResult[];
	
	    static createFrom(source: any = {}) {
	        return new BatchSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.succeeded = source["succeeded"];
	        this.failedAttempts = source["failedAttempts"];
	        this.currentStage = source["currentStage"];
	        this.currentEmail = source["currentEmail"];
	        this.running = source["running"];
	        this.updatedAt = source["updatedAt"];
	        this.results = this.convertValues(source["results"], AccountResult);
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
	export class Config {
	    panelBase: string;
	    panelKey: string;
	    panel2Base: string;
	    panel2Key: string;
	    mailMode: string;
	    mailDomain: string;
	    mailPrefix: string;
	    imapHost: string;
	    imapPort: number;
	    imapUser: string;
	    imapPassword: string;
	    hotmailRaw: string;
	    hotmailPool: string;
	    hotmailPreflightWorkers: number;
	    proxyUrl: string;
	    cursorProxy?: string;
	    proxyDirectHosts: string[];
	    proxyRegionRotate: string;
	    proxySessTime: number;
	    proxyStyle: string;
	    proxyState: string;
	    proxySticky?: boolean;
	    proxyRotateSession?: boolean;
	    proxySubscriptionUrl: string;
	    proxyPoolFile: string;
	    proxyPoolUse: boolean;
	    proxyPoolWorkers: number;
	    proxyPoolCheckUrl: string;
	    proxyLocalPorts: string;
	    ticketMode: string;
	    externalTicketUrl: string;
	    count: number;
	    workers: number;
	    platformMode: string;
	    cursorCount: number;
	    cursorWorkers: number;
	    captchaDirect: boolean;
	    mailViaProxy: boolean;
	    signinAfterRegister: boolean;
	    timezoneId: string;
	    chromePath: string;
	    outputDir: string;
	    signatureSalt: string;
	    yySuffix: string;
	    recaptchaSiteKey: string;
	    fixedPassword: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.panelBase = source["panelBase"];
	        this.panelKey = source["panelKey"];
	        this.panel2Base = source["panel2Base"];
	        this.panel2Key = source["panel2Key"];
	        this.mailMode = source["mailMode"];
	        this.mailDomain = source["mailDomain"];
	        this.mailPrefix = source["mailPrefix"];
	        this.imapHost = source["imapHost"];
	        this.imapPort = source["imapPort"];
	        this.imapUser = source["imapUser"];
	        this.imapPassword = source["imapPassword"];
	        this.hotmailRaw = source["hotmailRaw"];
	        this.hotmailPool = source["hotmailPool"];
	        this.hotmailPreflightWorkers = source["hotmailPreflightWorkers"];
	        this.proxyUrl = source["proxyUrl"];
	        this.cursorProxy = source["cursorProxy"];
	        this.proxyDirectHosts = source["proxyDirectHosts"];
	        this.proxyRegionRotate = source["proxyRegionRotate"];
	        this.proxySessTime = source["proxySessTime"];
	        this.proxyStyle = source["proxyStyle"];
	        this.proxyState = source["proxyState"];
	        this.proxySticky = source["proxySticky"];
	        this.proxyRotateSession = source["proxyRotateSession"];
	        this.proxySubscriptionUrl = source["proxySubscriptionUrl"];
	        this.proxyPoolFile = source["proxyPoolFile"];
	        this.proxyPoolUse = source["proxyPoolUse"];
	        this.proxyPoolWorkers = source["proxyPoolWorkers"];
	        this.proxyPoolCheckUrl = source["proxyPoolCheckUrl"];
	        this.proxyLocalPorts = source["proxyLocalPorts"];
	        this.ticketMode = source["ticketMode"];
	        this.externalTicketUrl = source["externalTicketUrl"];
	        this.count = source["count"];
	        this.workers = source["workers"];
	        this.platformMode = source["platformMode"];
	        this.cursorCount = source["cursorCount"];
	        this.cursorWorkers = source["cursorWorkers"];
	        this.captchaDirect = source["captchaDirect"];
	        this.mailViaProxy = source["mailViaProxy"];
	        this.signinAfterRegister = source["signinAfterRegister"];
	        this.timezoneId = source["timezoneId"];
	        this.chromePath = source["chromePath"];
	        this.outputDir = source["outputDir"];
	        this.signatureSalt = source["signatureSalt"];
	        this.yySuffix = source["yySuffix"];
	        this.recaptchaSiteKey = source["recaptchaSiteKey"];
	        this.fixedPassword = source["fixedPassword"];
	    }
	}
	export class CursorProgress {
	    target: number;
	    succeeded: number;
	    failed: number;
	    currentEmail: string;
	    running: boolean;
	    updatedAt: string;
	    lastPanelUser: string;
	
	    static createFrom(source: any = {}) {
	        return new CursorProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.succeeded = source["succeeded"];
	        this.failed = source["failed"];
	        this.currentEmail = source["currentEmail"];
	        this.running = source["running"];
	        this.updatedAt = source["updatedAt"];
	        this.lastPanelUser = source["lastPanelUser"];
	    }
	}
	export class HotmailPoolStats {
	    total: number;
	    available: number;
	    reserved: number;
	    used: number;
	    skipped: number;
	    byPlatform?: Record<string, HotmailPoolStats>;
	
	    static createFrom(source: any = {}) {
	        return new HotmailPoolStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.available = source["available"];
	        this.reserved = source["reserved"];
	        this.used = source["used"];
	        this.skipped = source["skipped"];
	        this.byPlatform = this.convertValues(source["byPlatform"], HotmailPoolStats, true);
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
	export class MailInboxItem {
	    id: string;
	    subject: string;
	    from: string;
	    receivedAt: string;
	    preview: string;
	    isRead: boolean;
	    folder: string;
	
	    static createFrom(source: any = {}) {
	        return new MailInboxItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.subject = source["subject"];
	        this.from = source["from"];
	        this.receivedAt = source["receivedAt"];
	        this.preview = source["preview"];
	        this.isRead = source["isRead"];
	        this.folder = source["folder"];
	    }
	}
	export class MailInboxView {
	    email: string;
	    items: MailInboxItem[];
	    tokenOk: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new MailInboxView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.items = this.convertValues(source["items"], MailInboxItem);
	        this.tokenOk = source["tokenOk"];
	        this.error = source["error"];
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
	export class MailMessageFull {
	    email: string;
	    subject: string;
	    from: string;
	    to: string;
	    receivedAt: string;
	    bodyHtml: string;
	    bodyText: string;
	
	    static createFrom(source: any = {}) {
	        return new MailMessageFull(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.subject = source["subject"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.receivedAt = source["receivedAt"];
	        this.bodyHtml = source["bodyHtml"];
	        this.bodyText = source["bodyText"];
	    }
	}
	export class MailboxOption {
	    email: string;
	    status: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new MailboxOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.status = source["status"];
	        this.note = source["note"];
	    }
	}
	export class PlatformStatus {
	    mode: string;
	    modeLabel: string;
	    miniMax: boolean;
	    cursor: boolean;
	    miniMaxRunning: boolean;
	    supervised: boolean;
	    cursorRunning: boolean;
	    cursorProgress: CursorProgress;
	
	    static createFrom(source: any = {}) {
	        return new PlatformStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.modeLabel = source["modeLabel"];
	        this.miniMax = source["miniMax"];
	        this.cursor = source["cursor"];
	        this.miniMaxRunning = source["miniMaxRunning"];
	        this.supervised = source["supervised"];
	        this.cursorRunning = source["cursorRunning"];
	        this.cursorProgress = this.convertValues(source["cursorProgress"], CursorProgress);
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
	export class ProgressLite {
	    target: number;
	    succeeded: number;
	    failedAttempts: number;
	    currentStage: string;
	    currentEmail: string;
	    running: boolean;
	    updatedAt: string;
	    points: number;
	    firstAt: string;
	    lastAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgressLite(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.succeeded = source["succeeded"];
	        this.failedAttempts = source["failedAttempts"];
	        this.currentStage = source["currentStage"];
	        this.currentEmail = source["currentEmail"];
	        this.running = source["running"];
	        this.updatedAt = source["updatedAt"];
	        this.points = source["points"];
	        this.firstAt = source["firstAt"];
	        this.lastAt = source["lastAt"];
	    }
	}
	export class ProxyPoolStats {
	    total: number;
	    alive: number;
	    dead: number;
	    checked: number;
	
	    static createFrom(source: any = {}) {
	        return new ProxyPoolStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.alive = source["alive"];
	        this.dead = source["dead"];
	        this.checked = source["checked"];
	    }
	}
	export class ResultsPage {
	    total: number;
	    offset: number;
	    limit: number;
	    rows: AccountResult[];
	
	    static createFrom(source: any = {}) {
	        return new ResultsPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	        this.rows = this.convertValues(source["rows"], AccountResult);
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

