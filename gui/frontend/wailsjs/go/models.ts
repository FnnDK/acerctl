export namespace main {
	
	export class FanSetting {
	    mode: string;
	    speed?: number;
	
	    static createFrom(source: any = {}) {
	        return new FanSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.speed = source["speed"];
	    }
	}
	export class Scenario {
	    name: string;
	    mode_ac?: string;
	    mode_battery?: string;
	    fans?: Record<string, FanSetting>;
	    kbd_timeout_s?: number;
	    apps?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Scenario(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode_ac = source["mode_ac"];
	        this.mode_battery = source["mode_battery"];
	        this.fans = this.convertValues(source["fans"], FanSetting, true);
	        this.kbd_timeout_s = source["kbd_timeout_s"];
	        this.apps = source["apps"];
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
	export class Titlebar {
	    left: string[];
	    right: string[];
	    icons: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Titlebar(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.left = source["left"];
	        this.right = source["right"];
	        this.icons = source["icons"];
	    }
	}

}

