{{define "js-browser-id"}}

var fmt = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx';

function generateUuid() {
    return fmt.replace(/[xy]/g, function(c) {
        var r = Math.random()*16|0, v = c === 'x' ? r : (r&0x3|0x8);
        return v.toString(16);
    });
}

function get_browser(){
    var ua=navigator.userAgent,tem,M=ua.match(/(opera|chrome|safari|firefox|msie|trident(?=\/))\/?\s*(\d+)/i) || []; 
    if(/trident/i.test(M[1])){
        tem=/\brv[ :]+(\d+)/g.exec(ua) || []; 
        return {name:'IE',version:(tem[1]||'')};
    }   
    if(M[1]==='Chrome'){
        tem=ua.match(/\bOPR\/(\d+)/)
        if(tem!=null)   {return {name:'Opera', version:tem[1]};}
    }   
    M=M[2]? [M[1], M[2]]: [navigator.appName, navigator.appVersion, '-?'];
    if((tem=ua.match(/version\/(\d+)/i))!=null) {M.splice(1,1,tem[1]);}
    return {
        name: M[0],
        version: M[1]
    };
}

var browser_uuid = localStorage.getItem('browser_uuid');
if (!browser_uuid) {
    browser_uuid = generateUuid();
    localStorage.setItem('browser_uuid', browser_uuid);
}

var browser = get_browser();
document.getElementById('browser_uuid').value = browser_uuid;
document.getElementById('browser_name').value = browser.name;
document.getElementById('browser_version').value = browser.version;
document.getElementById('browser_vendor').value = navigator.vendor;
document.getElementById('browser_platform').value = navigator.platform;
{{end}}
