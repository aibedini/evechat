(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const a of document.querySelectorAll('link[rel="modulepreload"]'))i(a);new MutationObserver(a=>{for(const r of a)if(r.type==="childList")for(const c of r.addedNodes)c.tagName==="LINK"&&c.rel==="modulepreload"&&i(c)}).observe(document,{childList:!0,subtree:!0});function n(a){const r={};return a.integrity&&(r.integrity=a.integrity),a.referrerPolicy&&(r.referrerPolicy=a.referrerPolicy),a.crossOrigin==="use-credentials"?r.credentials="include":a.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function i(a){if(a.ep)return;a.ep=!0;const r=n(a);fetch(a.href,r)}})();const m=document.getElementById("app"),e={step:1,status:null,checks:[],admin:{name:"مدیر سیستم",email:"",password:""},project:{name:"EVE",domains:[]},domainInput:"",integrations:{},results:{},result:null,error:"",busy:!1,setupToken:localStorage.getItem("evechat:setup-token")||""},h=["خوش آمدید","حساب مدیر","اولین پروژه","یکپارچه‌سازی‌ها","پایان"];function o(s){return String(s).replace(/[&<>"']/g,t=>t==="&"?"&amp;":t==="<"?"&lt;":t===">"?"&gt;":t==='"'?"&quot;":"&#39;")}async function l(s,t="GET",n){const i={Accept:"application/json"};n!==void 0&&(i["Content-Type"]="application/json"),e.setupToken&&(i["X-Setup-Token"]=e.setupToken);const a=await fetch(s,{method:t,headers:i,body:n===void 0?void 0:JSON.stringify(n)}),r=await a.text();let c=null;try{c=r?JSON.parse(r):null}catch(u){c=null}if(!a.ok)throw new Error((c==null?void 0:c.message)||`خطای سرور (${a.status})`);return c}function y(){return`<div class="brand">
    <div class="brand-mark">⚡</div>
    <div><h1>EveChat</h1><p>راه‌اندازی اولیه ${e.status?"— نسخه "+o(e.status.version):""}</p></div>
  </div>
  <div class="progress">${h.map((s,t)=>`<span class="${t+1<=e.step?"on":""}"></span>`).join("")}</div>`}function v(){return e.error?`<div class="error">${o(e.error)}</div>`:""}function g(s){const t=s.ok?'<span class="good">✓</span>':`<span class="${s.fatal?"bad":"muted"}">${s.fatal?"✗":"!"}</span>`,n=!s.ok&&s.fix?`<div class="fix">راه‌حل: ${o(s.fix)}</div>`:"";return`<div class="check"><span class="icon">${t}</span><div><div>${o(s.name)} — ${o(s.message)}</div>${n}</div></div>`}function k(){const s=e.checks.some(t=>!t.ok&&t.fatal);return`<div class="card">
    <h2>به EveChat خوش آمدید</h2>
    <p class="sub">در چند گام کوتاه، سرور، حساب مدیر و اولین پروژه را آماده می‌کنیم.</p>
    <div>${e.checks.length?e.checks.map(g).join(""):'<div class="muted">در حال بررسی…</div>'}</div>
    <div class="actions">
      <button class="btn btn-primary" id="start" ${s?"disabled":""}>شروع</button>
      <button class="btn btn-ghost" id="recheck">بررسی مجدد</button>
      <span class="spacer"></span>
    </div>
    <div class="field" style="margin-top:18px">
      <label>توکن نصب (فقط برای نصب از راه دور لازم است)</label>
      <input id="setupToken" dir="ltr" placeholder="از خروجی سرور یا setup-token.txt" value="${o(e.setupToken)}" />
    </div>
    ${v()}
  </div>`}function $(s){let t=0;return s.length>=10&&t++,s.length>=14&&t++,/[A-Z]/.test(s)&&t++,/[0-9]/.test(s)&&t++,/[^A-Za-z0-9]/.test(s)&&t++,Math.min(t,5)}function w(){return`<div class="card">
    <h2>حساب مدیر</h2>
    <p class="sub">این حساب به همه بخش‌ها دسترسی دارد.</p>
    <div class="field"><label>نام</label><input id="adminName" value="${o(e.admin.name)}" /></div>
    <div class="field"><label>ایمیل</label><input id="adminEmail" dir="ltr" type="email" value="${o(e.admin.email)}" placeholder="admin@example.com" /></div>
    <div class="field"><label>رمز عبور (حداقل ۸ کاراکتر)</label>
      <input id="adminPassword" dir="ltr" type="password" value="${o(e.admin.password)}" />
      <div class="strength"><i id="strengthBar"></i></div>
      <button class="btn-link" id="genPassword" style="margin-top:6px">تولید رمز قوی</button>
    </div>
    ${v()}
    <div class="actions"><button class="btn btn-primary" id="next">ادامه</button></div>
  </div>`}function x(){return`<div class="card">
    <h2>اولین پروژه</h2>
    <p class="sub">پروژه، کلید ویجت و دامنه‌های مجاز را مشخص می‌کند.</p>
    <div class="field"><label>نام پروژه</label><input id="projectName" value="${o(e.project.name)}" /></div>
    <div class="field"><label>دامنه‌های مجاز (Enter برای افزودن)</label>
      <input id="domainInput" dir="ltr" placeholder="eve.ir" value="${o(e.domainInput)}" />
      <div class="chips">${e.project.domains.map((s,t)=>`<span class="chip">${o(s)}<button data-remove="${t}">×</button></span>`).join("")}</div>
      ${e.project.domains.length===0?'<div class="muted" style="font-size:11px;margin-top:6px">خالی = حالت توسعه (ویجت روی همه دامنه‌ها)</div>':""}
    </div>
    <div class="preview">
      <header>${o(e.project.name||"EVE")}</header>
      <div class="body">سلام 👋 چطور می‌توانیم کمک کنیم؟</div>
    </div>
    ${v()}
    <div class="actions">
      <button class="btn btn-ghost" id="back">بازگشت</button>
      <button class="btn btn-primary" id="next">ادامه</button>
    </div>
  </div>`}function p(s,t,n){const i=e.integrations[s]||{},a=e.results[s];return`<details class="accordion" ${a?"open":""}>
    <summary>${t} ${a?a.ok?'<span class="good">✓</span>':'<span class="bad">✗</span>':""}</summary>
    <div class="body">
      ${n.map(([r,c,u])=>`<div class="field"><label>${c}</label><input data-int="${s}" data-field="${r}" type="${u}" dir="ltr" value="${o(i[r]||"")}" /></div>`).join("")}
      <div class="actions">
        <button class="btn btn-ghost" data-test="${s}">تست اتصال</button>
        ${a?`<span class="${a.ok?"good":"bad"}" style="font-size:12px">${o(a.message)}</span>`:""}
      </div>
    </div>
  </details>`}function E(){return`<div class="card">
    <h2>یکپارچه‌سازی‌ها</h2>
    <p class="sub">همه اختیاری هستند؛ هر زمان از پنل قابل تغییرند.</p>
    ${p("telegram","تلگرام",[["token","توکن ربات (BotFather)","password"],["secret","کلید وب‌هوک (اختیاری)","text"]])}
    ${p("eve","EVE",[["base_url","آدرس EVE","text"],["api_key","کلید API","password"]])}
    ${p("gmweb","GMweb (پیامک)",[["base_url","آدرس درگاه","text"],["api_key","کلید API","password"]])}
    ${p("ai","دستیار هوش مصنوعی",[["api_key","کلید Anthropic","password"],["model","مدل","text"]])}
    ${p("smtp","ایمیل (SMTP)",[["host","Host","text"],["port","Port","number"],["username","Username","text"],["password","Password","password"],["from_email","From","text"]])}
    ${v()}
    <div class="actions">
      <button class="btn btn-ghost" id="back">بازگشت</button>
      <button class="btn btn-primary" id="next">ادامه</button>
      <button class="btn-link" id="skip">رد کردن</button>
    </div>
  </div>`}function j(){const s=e.result;return s?`<div class="card">
    <h2>EveChat آماده است 🎉</h2>
    <p class="sub">اطلاعات زیر را جای امنی ذخیره کنید.</p>
    <div class="success-box">
      <div class="cred"><span>پنل</span><code>${o(s.dashboard_url)}</code></div>
      <div class="cred"><span>ایمیل</span><code>${o(s.admin_email)}</code></div>
      ${s.admin_password?`<div class="cred"><span>رمز عبور</span><code>${o(s.admin_password)}</code></div>`:""}
      <div class="cred"><span>کلید پروژه</span><code>${o(s.project_key)}</code></div>
    </div>
    <h2 style="font-size:15px">کد جاسازی ویجت</h2>
    <pre id="snippet">${o(s.embed_snippet)}</pre>
    <div class="actions">
      <button class="btn btn-ghost" id="copySnippet">کپی کد</button>
      <button class="btn btn-ghost" id="downloadSummary">دانلود خلاصه</button>
      <span class="spacer"></span>
      <button class="btn btn-primary" id="enter">ورود به پنل</button>
    </div>
    <p class="muted" style="font-size:11px;margin-top:10px">برای امنیت، فایل <code>setup-token.txt</code> را پس از نصب حذف کنید.</p>
  </div>`:'<div class="card"><h2>پایان</h2><p class="sub">در حال نهایی‌سازی…</p></div>'}function d(){const s=e.step===1?k():e.step===2?w():e.step===3?x():e.step===4?E():j();m.innerHTML=`<div class="shell">${y()}${s}</div>`,S()}function S(){const s=(t,n,i)=>{const a=document.getElementById(t);a&&a.addEventListener(n,i)};if(e.step===1){const t=document.getElementById("setupToken");t==null||t.addEventListener("input",()=>{e.setupToken=t.value.trim(),localStorage.setItem("evechat:setup-token",e.setupToken)}),s("start","click",()=>{e.step=2,e.error="",d()}),s("recheck","click",async()=>{e.error="",await b(!0)});return}if(e.step===2){const t=document.getElementById("adminPassword"),n=document.getElementById("strengthBar"),i=()=>{const a=$(t.value);n.style.width=a/5*100+"%",n.style.background=a>=4?"#10b981":a>=3?"#f59e0b":"#ef4444"};t.addEventListener("input",i),i(),s("genPassword","click",()=>{var c;const a="abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$",r=new Uint8Array(16);crypto.getRandomValues(r),t.value=Array.from(r,u=>a[u%a.length]).join(""),i(),(c=navigator.clipboard)==null||c.writeText(t.value).catch(()=>{})}),s("next","click",async()=>{if(e.admin.name=document.getElementById("adminName").value.trim(),e.admin.email=document.getElementById("adminEmail").value.trim(),e.admin.password=t.value,e.error="",!e.admin.email.includes("@")){e.error="ایمیل معتبر نیست",d();return}if(e.admin.password.length<8){e.error="رمز عبور باید حداقل ۸ کاراکتر باشد",d();return}e.busy=!0;try{await l("/api/setup/admin","POST",e.admin),e.step=3}catch(a){e.error=a.message}finally{e.busy=!1,d()}});return}if(e.step===3){const t=document.getElementById("domainInput");t==null||t.addEventListener("keydown",n=>{if(n.key!=="Enter")return;n.preventDefault();const i=t.value.trim().toLowerCase();i&&!e.project.domains.includes(i)&&e.project.domains.push(i),t.value="",e.project.name=document.getElementById("projectName").value,d()}),m.querySelectorAll("[data-remove]").forEach(n=>{n.addEventListener("click",()=>{const i=Number(n.dataset.remove);e.project.domains.splice(i,1),e.project.name=document.getElementById("projectName").value,d()})}),s("back","click",()=>{e.step=2,d()}),s("next","click",async()=>{e.project.name=document.getElementById("projectName").value.trim(),e.error="",e.busy=!0;try{await l("/api/setup/project","POST",{name:e.project.name,domains:e.project.domains}),e.step=4}catch(n){e.error=n.message}finally{e.busy=!1,d()}});return}if(e.step===4){m.querySelectorAll("[data-int]").forEach(t=>{t.addEventListener("input",()=>{const n=t,i=n.dataset.int,a=n.dataset.field;e.integrations[i]=e.integrations[i]||{},e.integrations[i][a]=n.value})}),m.querySelectorAll("[data-test]").forEach(t=>{t.addEventListener("click",async()=>{const n=t.dataset.test,i=e.integrations[n]||{};try{const a=await l(`/api/setup/test/${n}`,"POST",i);e.results[n]=a}catch(a){e.results[n]={ok:!1,message:a.message}}d()})}),s("back","click",()=>{e.step=3,d()}),s("skip","click",()=>f()),s("next","click",async()=>{e.error="",e.busy=!0;try{const t={};for(const[n,i]of Object.entries(e.integrations))Object.values(i).some(a=>a.trim()!=="")&&(t[n]=i);if(Object.keys(t).length>0){const n=await l("/api/setup/integrations","POST",t);e.results={...e.results,...n.results},Object.values(n.results).filter(a=>!a.ok).length>0&&(e.error="ذخیره شد ولی تست برخی سرویس‌ها ناموفق بود — بعداً در تنظیمات اصلاح کنید.")}await f()}catch(t){e.error=t.message}finally{e.busy=!1,d()}});return}s("copySnippet","click",()=>{var n,i;(i=navigator.clipboard)==null||i.writeText(((n=e.result)==null?void 0:n.embed_snippet)||"").catch(()=>{});const t=document.getElementById("copySnippet");t&&(t.textContent="کپی شد ✓")}),s("downloadSummary","click",()=>{const t=e.result;if(!t)return;const n=`EveChat setup

Dashboard: ${t.dashboard_url}
Email: ${t.admin_email}
Password: ${t.admin_password||"(not shown)"}
Project key: ${t.project_key}

${t.embed_snippet}
`,i=new Blob([n],{type:"text/plain;charset=utf-8"}),a=document.createElement("a");a.href=URL.createObjectURL(i),a.download="evechat-setup.txt",a.click(),URL.revokeObjectURL(a.href)}),s("enter","click",()=>{const t=localStorage.getItem("evechat:access");location.assign(t?"/inbox":"/login")})}async function f(){try{const s=await l("/api/setup/complete","POST",{});localStorage.setItem("evechat:access",s.access_token),localStorage.setItem("evechat:refresh",s.refresh_token);const t=await l("/api/v1/auth/me").catch(()=>null);e.result={admin_email:(t==null?void 0:t.admin_email)||e.admin.email,admin_password:e.admin.password,project_key:await _()||"",dashboard_url:location.origin,embed_snippet:await P()||""},e.step=5}catch(s){e.error=s.message}d()}async function _(){try{return(await l("/api/v1/settings/project")).project.api_key}catch(s){return""}}async function P(){try{return(await l("/api/v1/settings/project")).embed_code}catch(s){return""}}async function b(s=!1){try{if(s){const n=await l("/api/setup/check","POST",{});e.checks=n.checks.map(i=>({id:i.id,name:i.name,ok:i.result.ok,message:i.result.message,fix:i.result.fix,fatal:i.level===0})),d();return}const t=await l("/api/setup/status");if(e.status=t,e.checks=t.checks||[],!t.setup_required){location.assign("/inbox");return}e.step=t.admin_exists?t.project_exists?4:3:1,d(),e.step===1&&e.checks.length===0&&await b(!0)}catch(t){e.error=t.message,d()}}b();
