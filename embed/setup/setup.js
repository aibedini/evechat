(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const a of document.querySelectorAll('link[rel="modulepreload"]'))r(a);new MutationObserver(a=>{for(const i of a)if(i.type==="childList")for(const d of i.addedNodes)d.tagName==="LINK"&&d.rel==="modulepreload"&&r(d)}).observe(document,{childList:!0,subtree:!0});function n(a){const i={};return a.integrity&&(i.integrity=a.integrity),a.referrerPolicy&&(i.referrerPolicy=a.referrerPolicy),a.crossOrigin==="use-credentials"?i.credentials="include":a.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function r(a){if(a.ep)return;a.ep=!0;const i=n(a);fetch(a.href,i)}})();class h extends Error{constructor(t,n){super(t),this.status=n}}const v=document.getElementById("app"),e={step:1,status:null,checks:[],admin:{name:"مدیر سیستم",email:"",password:""},project:{name:"EVE",domains:[]},domainInput:"",integrations:{},results:{},result:null,error:"",busy:!1,setupToken:localStorage.getItem("evechat:setup-token")||"",tokenRequired:!1},y={"fs.data":"فضای ذخیره‌سازی","db.open":"پایگاه داده","net.port":"پورت","config.keys":"کلیدهای امنیتی","time.skew":"ساعت سرور","net.outbound":"دسترسی اینترنت (اختیاری)","assets.font":"فونت","assets.dashboard":"فایل‌های پنل"},g=["خوش آمدید","حساب مدیر","اولین پروژه","یکپارچه‌سازی‌ها","پایان"];function o(s){return String(s).replace(/[&<>"']/g,t=>t==="&"?"&amp;":t==="<"?"&lt;":t===">"?"&gt;":t==='"'?"&quot;":"&#39;")}async function l(s,t="GET",n){const r={Accept:"application/json"};n!==void 0&&(r["Content-Type"]="application/json"),e.setupToken&&(r["X-Setup-Token"]=e.setupToken);const a=await fetch(s,{method:t,headers:r,body:n===void 0?void 0:JSON.stringify(n)}),i=await a.text();let d=null;try{d=i?JSON.parse(i):null}catch(m){d=null}if(!a.ok)throw new h((d==null?void 0:d.message)||`خطای سرور (${a.status})`,a.status);return d}function k(){return`<div class="brand">
    <div class="brand-mark">⚡</div>
    <div><h1>EveChat</h1><p>راه‌اندازی اولیه ${e.status?"— نسخه "+o(e.status.version):""}</p></div>
  </div>
  <div class="progress">${g.map((s,t)=>`<span class="${t+1<=e.step?"on":""}"></span>`).join("")}</div>`}function p(){return e.error?`<div class="error">${o(e.error)}</div>`:""}function $(s){const t=s.ok?'<span class="good">✓</span>':`<span class="${s.fatal?"bad":"muted"}">${s.fatal?"✗":"!"}</span>`,n=!s.ok&&s.fix?`<div class="fix">راه‌حل: ${o(s.fix)}</div>`:"",r=y[s.id]||s.name;return`<div class="check"><span class="icon">${t}</span><div><div>${o(r)} — ${o(s.message)}</div>${n}</div></div>`}function w(){if(e.tokenRequired)return`<div class="card" data-testid="setup-token-gate">
      <h2>رمز نصب را وارد کنید</h2>
      <p class="sub">
        برای اینکه فقط صاحب سرور بتواند نصب را تمام کند، این رمز لازم است. آن را در فایل
        <code>data/setup-token.txt</code> کنار پایگاه داده، یا در لاگ سرور
        (برای داکر: <code>docker logs evechat</code>) پیدا می‌کنید.
      </p>
      <div class="field">
        <label for="setupToken">رمز نصب</label>
        <input id="setupToken" dir="ltr" autocomplete="off" spellcheck="false"
          placeholder="از فایل setup-token.txt یا لاگ سرور" value="${o(e.setupToken)}" />
      </div>
      ${p()}
      <div class="actions">
        <button class="btn btn-primary" id="checkToken">بررسی رمز</button>
      </div>
    </div>`;const s=e.checks.some(n=>!n.ok&&n.fatal),t=e.status!==null;return`<div class="card">
    <h2>خوش آمدید</h2>
    <p class="sub">چند دقیقه‌ی دیگر چت پشتیبانی‌تان آماده است. اول بررسی می‌کنیم سرور سالم است.</p>
    <div>${e.checks.length?e.checks.map($).join(""):'<div class="muted">در حال بررسی…</div>'}</div>
    <div class="actions">
      <button class="btn btn-primary" id="start" ${s||!t?"disabled":""}>شروع</button>
      <button class="btn btn-ghost" id="recheck">بررسی مجدد</button>
      <span class="spacer"></span>
    </div>
    ${p()}
  </div>`}function x(s){let t=0;return s.length>=10&&t++,s.length>=14&&t++,/[A-Z]/.test(s)&&t++,/[0-9]/.test(s)&&t++,/[^A-Za-z0-9]/.test(s)&&t++,Math.min(t,5)}function E(){return`<div class="card">
    <h2>حساب مدیر</h2>
    <p class="sub">این حساب به همه‌چیز دسترسی دارد. رمز را جای امنی نگه دارید.</p>
    <div class="field"><label>نام</label><input id="adminName" value="${o(e.admin.name)}" /></div>
    <div class="field"><label>ایمیل</label><input id="adminEmail" dir="ltr" type="email" value="${o(e.admin.email)}" placeholder="admin@example.com" /></div>
    <div class="field"><label>رمز عبور (حداقل ۸ کاراکتر)</label>
      <input id="adminPassword" dir="ltr" type="password" value="${o(e.admin.password)}" />
      <div class="strength"><i id="strengthBar"></i></div>
      <button class="btn-link" id="genPassword" style="margin-top:6px">تولید رمز قوی</button>
    </div>
    ${p()}
    <div class="actions"><button class="btn btn-primary" id="next">ادامه</button></div>
  </div>`}function j(){return`<div class="card">
    <h2>اولین پروژه</h2>
    <p class="sub">پروژه یعنی یک سایت یا سرویس شما. «دامنه‌های مجاز»: فقط از این سایت‌ها ویجت کار می‌کند. اگر خالی بگذارید از هر سایتی کار می‌کند؛ این فقط برای آزمایش است.</p>
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
    ${p()}
    <div class="actions">
      <button class="btn btn-ghost" id="back">بازگشت</button>
      <button class="btn btn-primary" id="next">ادامه</button>
    </div>
  </div>`}function u(s,t,n){const r=e.integrations[s]||{},a=e.results[s];return`<details class="accordion" ${a?"open":""}>
    <summary>${t} ${a?a.ok?'<span class="good">✓</span>':'<span class="bad">✗</span>':""}</summary>
    <div class="body">
      ${n.map(([i,d,m])=>`<div class="field"><label>${d}</label><input data-int="${s}" data-field="${i}" type="${m}" dir="ltr" value="${o(r[i]||"")}" /></div>`).join("")}
      <div class="actions">
        <button class="btn btn-ghost" data-test="${s}">تست اتصال</button>
        ${a?`<span class="${a.ok?"good":"bad"}" style="font-size:12px">${o(a.message)}</span>`:""}
      </div>
    </div>
  </details>`}function S(){return`<div class="card">
    <h2>یکپارچه‌سازی‌ها</h2>
    <p class="sub">همه اختیاری‌اند و بعداً هم از تنظیمات می‌شود اضافه کرد.</p>
    ${u("telegram","تلگرام",[["token","توکن ربات (BotFather)","password"],["secret","کلید وب‌هوک (اختیاری)","text"]])}
    ${u("eve","EVE",[["base_url","آدرس EVE","text"],["api_key","کلید API","password"]])}
    ${u("gmweb","GMweb (پیامک)",[["base_url","آدرس درگاه","text"],["api_key","کلید API","password"]])}
    ${u("ai","دستیار هوش مصنوعی",[["api_key","کلید Anthropic","password"],["model","مدل","text"]])}
    ${u("smtp","ایمیل (SMTP)",[["host","Host","text"],["port","Port","number"],["username","Username","text"],["password","Password","password"],["from_email","From","text"]])}
    ${p()}
    <div class="actions">
      <button class="btn btn-ghost" id="back">بازگشت</button>
      <button class="btn btn-primary" id="next">ادامه</button>
      <button class="btn-link" id="skip">رد کردن</button>
    </div>
  </div>`}function _(){const s=e.result;return s?`<div class="card">
    <h2>آماده است!</h2>
    <p class="sub">کد زیر را قبل از <code>&lt;/body&gt;</code> سایت‌تان بچسبانید. اطلاعات ورود را هم جای امنی ذخیره کنید.</p>
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
  </div>`:'<div class="card"><h2>پایان</h2><p class="sub">در حال نهایی‌سازی…</p></div>'}function c(){const s=e.step===1?w():e.step===2?E():e.step===3?j():e.step===4?S():_();v.innerHTML=`<div class="shell">${k()}${s}</div>`,T()}function T(){const s=(t,n,r)=>{const a=document.getElementById(t);a&&a.addEventListener(n,r)};if(e.step===1){const t=document.getElementById("setupToken");if(t==null||t.addEventListener("input",()=>{e.setupToken=t.value.trim(),localStorage.setItem("evechat:setup-token",e.setupToken),e.error=""}),e.tokenRequired){const n=async()=>{if(e.setupToken=((t==null?void 0:t.value)||"").trim(),localStorage.setItem("evechat:setup-token",e.setupToken),!e.setupToken){e.error="رمز نصب را وارد کنید.",c();return}await b()};s("checkToken","click",n),t==null||t.addEventListener("keydown",r=>{r.key==="Enter"&&(r.preventDefault(),n())});return}s("start","click",()=>{e.step=2,e.error="",c()}),s("recheck","click",async()=>{e.error="",await b(!0)});return}if(e.step===2){const t=document.getElementById("adminPassword"),n=document.getElementById("strengthBar"),r=()=>{const a=x(t.value);n.style.width=a/5*100+"%",n.style.background=a>=4?"#10b981":a>=3?"#f59e0b":"#ef4444"};t.addEventListener("input",r),r(),s("genPassword","click",()=>{var d;const a="abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$",i=new Uint8Array(16);crypto.getRandomValues(i),t.value=Array.from(i,m=>a[m%a.length]).join(""),r(),(d=navigator.clipboard)==null||d.writeText(t.value).catch(()=>{})}),s("next","click",async()=>{if(e.admin.name=document.getElementById("adminName").value.trim(),e.admin.email=document.getElementById("adminEmail").value.trim(),e.admin.password=t.value,e.error="",!e.admin.email.includes("@")){e.error="ایمیل معتبر نیست",c();return}if(e.admin.password.length<8){e.error="رمز عبور باید حداقل ۸ کاراکتر باشد",c();return}e.busy=!0;try{await l("/api/setup/admin","POST",e.admin),e.step=3}catch(a){e.error=a.message}finally{e.busy=!1,c()}});return}if(e.step===3){const t=document.getElementById("domainInput");t==null||t.addEventListener("keydown",n=>{if(n.key!=="Enter")return;n.preventDefault();const r=t.value.trim().toLowerCase();r&&!e.project.domains.includes(r)&&e.project.domains.push(r),t.value="",e.project.name=document.getElementById("projectName").value,c()}),v.querySelectorAll("[data-remove]").forEach(n=>{n.addEventListener("click",()=>{const r=Number(n.dataset.remove);e.project.domains.splice(r,1),e.project.name=document.getElementById("projectName").value,c()})}),s("back","click",()=>{e.step=2,c()}),s("next","click",async()=>{e.project.name=document.getElementById("projectName").value.trim(),e.error="",e.busy=!0;try{await l("/api/setup/project","POST",{name:e.project.name,domains:e.project.domains}),e.step=4}catch(n){e.error=n.message}finally{e.busy=!1,c()}});return}if(e.step===4){v.querySelectorAll("[data-int]").forEach(t=>{t.addEventListener("input",()=>{const n=t,r=n.dataset.int,a=n.dataset.field;e.integrations[r]=e.integrations[r]||{},e.integrations[r][a]=n.value})}),v.querySelectorAll("[data-test]").forEach(t=>{t.addEventListener("click",async()=>{const n=t.dataset.test,r=e.integrations[n]||{};try{const a=await l(`/api/setup/test/${n}`,"POST",r);e.results[n]=a}catch(a){e.results[n]={ok:!1,message:a.message}}c()})}),s("back","click",()=>{e.step=3,c()}),s("skip","click",()=>f()),s("next","click",async()=>{e.error="",e.busy=!0;try{const t={};for(const[n,r]of Object.entries(e.integrations))Object.values(r).some(a=>a.trim()!=="")&&(t[n]=r);if(Object.keys(t).length>0){const n=await l("/api/setup/integrations","POST",t);e.results={...e.results,...n.results},Object.values(n.results).filter(a=>!a.ok).length>0&&(e.error="ذخیره شد ولی تست برخی سرویس‌ها ناموفق بود — بعداً در تنظیمات اصلاح کنید.")}await f()}catch(t){e.error=t.message}finally{e.busy=!1,c()}});return}s("copySnippet","click",()=>{var n,r;(r=navigator.clipboard)==null||r.writeText(((n=e.result)==null?void 0:n.embed_snippet)||"").catch(()=>{});const t=document.getElementById("copySnippet");t&&(t.textContent="کپی شد ✓")}),s("downloadSummary","click",()=>{const t=e.result;if(!t)return;const n=`EveChat setup

Dashboard: ${t.dashboard_url}
Email: ${t.admin_email}
Password: ${t.admin_password||"(not shown)"}
Project key: ${t.project_key}

${t.embed_snippet}
`,r=new Blob([n],{type:"text/plain;charset=utf-8"}),a=document.createElement("a");a.href=URL.createObjectURL(r),a.download="evechat-setup.txt",a.click(),URL.revokeObjectURL(a.href)}),s("enter","click",()=>{const t=localStorage.getItem("evechat:access");location.assign(t?"/inbox":"/login")})}async function f(){try{const s=await l("/api/setup/complete","POST",{});localStorage.setItem("evechat:access",s.access_token),localStorage.setItem("evechat:refresh",s.refresh_token);const t=await l("/api/v1/auth/me").catch(()=>null);e.result={admin_email:(t==null?void 0:t.admin_email)||e.admin.email,admin_password:e.admin.password,project_key:await P()||"",dashboard_url:location.origin,embed_snippet:await L()||""},e.step=5}catch(s){e.error=s.message}c()}async function P(){try{return(await l("/api/v1/settings/project")).project.api_key}catch(s){return""}}async function L(){try{return(await l("/api/v1/settings/project")).embed_code}catch(s){return""}}async function b(s=!1){try{if(s){const n=await l("/api/setup/check","POST",{});e.checks=n.checks.map(r=>({id:r.id,name:r.name,ok:r.result.ok,message:r.result.message,fix:r.result.fix,fatal:r.level===0})),c();return}const t=await l("/api/setup/status");if(e.status=t,e.tokenRequired=!1,e.error="",e.checks=t.checks||[],!t.setup_required){location.assign("/inbox");return}e.step=t.admin_exists?t.project_exists?4:3:1,c(),e.step===1&&e.checks.length===0&&await b(!0)}catch(t){(t instanceof h?t.status:0)===401?(e.tokenRequired=!0,e.step=1,e.error=e.setupToken?"رمز درست نیست. دوباره از فایل کپی کنید؛ فاصله‌ی اضافه‌ی اول یا آخر را نگیرید.":""):e.error=t.message,c()}}b();
