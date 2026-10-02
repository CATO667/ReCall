import{c as V,a as z,f as M,b as E,u as D,e as F,d as L,g as U,h as R,p as K,i as Q,j as G,w as J,o as i,k as d,n as S,l as w,m as n,q as P,r as O,L as X,s as p,T as Y,t as Z,v as C,x as H,y as ee,z as te,A as h,B as c,C as k,D as _,F as q,E as N,G as g}from"./index-b428095f.js";import{u as se,I,T as ne}from"./Input-150a4216.js";var ae=V([V("@keyframes spin-rotate",`
 from {
 transform: rotate(0);
 }
 to {
 transform: rotate(360deg);
 }
 `),z("spin-container",`
 position: relative;
 `,[z("spin-body",`
 position: absolute;
 top: 50%;
 left: 50%;
 transform: translateX(-50%) translateY(-50%);
 `,[M()])]),z("spin-body",`
 display: inline-flex;
 align-items: center;
 justify-content: center;
 flex-direction: column;
 `),z("spin",`
 display: inline-flex;
 height: var(--n-size);
 width: var(--n-size);
 font-size: var(--n-size);
 color: var(--n-color);
 `,[E("rotate",`
 animation: spin-rotate 2s linear infinite;
 `)]),z("spin-description",`
 display: inline-block;
 font-size: var(--n-font-size);
 color: var(--n-text-color);
 transition: color .3s var(--n-bezier);
 margin-top: 8px;
 `),z("spin-content",`
 opacity: 1;
 transition: opacity .3s var(--n-bezier);
 pointer-events: all;
 `,[E("spinning",`
 user-select: none;
 -webkit-user-select: none;
 pointer-events: none;
 opacity: var(--n-opacity-spinning);
 `)])]);const ie={small:20,medium:18,large:16},oe={...D.props,contentClass:String,contentStyle:[Object,String],description:String,size:{type:[String,Number],default:"medium"},show:{type:Boolean,default:!0},rotate:{type:Boolean,default:!0},spinning:{type:Boolean,validator:()=>!0,default:void 0},delay:Number,...F,strokeWidth:Number};var re=L({name:"Spin",props:oe,slots:Object,setup(s){const{mergedClsPrefixRef:o,inlineThemeDisabled:f}=U(s),m=D("Spin","-spin",ae,Z,s,o),y=R(()=>{const{size:a}=s,{common:{cubicBezierEaseInOut:v},self:x}=m.value,{opacitySpinning:T,color:$,textColor:B}=x;return{"--n-bezier":v,"--n-opacity-spinning":T,"--n-size":typeof a=="number"?K(a):x[Q("size",a)],"--n-color":$,"--n-text-color":B}}),u=f?G("spin",R(()=>{const{size:a}=s;return typeof a=="number"?String(a):a[0]}),y,s):void 0,b=se(s,["spinning","show"]),l=C(!1);return J(a=>{let v;if(b.value){const{delay:x}=s;if(x){v=window.setTimeout(()=>{l.value=!0},x),a(()=>{clearTimeout(v)});return}}l.value=b.value}),{mergedClsPrefix:o,active:l,mergedStrokeWidth:R(()=>{const{strokeWidth:a}=s;if(a!==void 0)return a;const{size:v}=s;return ie[typeof v=="number"?"medium":v]}),cssVars:f?void 0:y,themeClass:u==null?void 0:u.themeClass,onRender:u==null?void 0:u.onRender}},render(){var b;const{$slots:s,mergedClsPrefix:o,description:f}=this,m=s.icon&&this.rotate,y=(f||s.description)&&(i(),d("div",{class:w(`${o}-spin-description`)},[S(()=>{var l;return f||((l=s.description)==null?void 0:l.call(s))})],2)),u=s.icon?(i(),d("div",{key:1,class:w([`${o}-spin-body`,this.themeClass])},[n("div",{class:w([`${o}-spin`,m&&`${o}-spin--rotate`]),style:P(s.default?"":this.cssVars)},[S(()=>s.icon())],6),S(()=>y)],2)):(i(),d("div",{key:2,class:w([`${o}-spin-body`,this.themeClass])},[(i(),O(X,{clsPrefix:o,style:P(s.default?"":this.cssVars),stroke:this.stroke,"stroke-width":this.mergedStrokeWidth,radius:this.radius,scale:this.scale,class:w(`${o}-spin`)},null,8,["clsPrefix","style","stroke","stroke-width","radius","scale","class"])),S(()=>y)],2));return(b=this.onRender)==null||b.call(this),s.default?(i(),d("div",{key:3,class:w([`${o}-spin-container`,this.themeClass]),style:P(this.cssVars)},[n("div",{class:w([`${o}-spin-content`,this.active&&`${o}-spin-content--spinning`,this.contentClass]),style:P(this.contentStyle)},[S(()=>{var l;return(l=s.default)==null?void 0:l.call(s)})],6),p(Y,{name:"fade-in-transition"},{default:()=>this.active?u:null},1024)],6)):u}});const le={class:"h-full flex flex-col bg-white dark:bg-[#24272e]"},de={class:"flex items-center justify-between px-4 h-14 border-b dark:border-neutral-800"},ue={class:"flex items-center gap-3"},ce={class:"max-w-5xl mx-auto p-4 space-y-8"},pe={class:"flex flex-wrap gap-2 mb-3"},fe={key:0,class:"text-gray-500 text-sm"},me={key:1,class:"space-y-2"},ve={class:"font-medium"},ye={class:"text-gray-500 mt-1"},he={class:"flex items-center justify-between mb-3"},ge={class:"flex gap-2"},xe={class:"flex flex-wrap gap-2 mb-3"},ke={class:"flex flex-wrap gap-2 mb-4"},be={key:0,class:"text-gray-500 text-sm"},we={key:1,class:"space-y-3"},_e={class:"font-medium mb-1"},Ce={class:"text-xs text-gray-500 mb-2"},ze={class:"whitespace-pre-wrap text-sm text-gray-700 dark:text-gray-300"},$e=L({__name:"index",setup(s){const o=H(),f=ee(),m=C(!1),y=C([]),u=C([]),b=C([]),l=C("data/kbase/samples/mykvstore-intro.md"),a=C("");async function v(){const r=await fetch("/api/kbase/documents"),e=await r.json();if(!r.ok)throw new Error(e.error||"加载文档失败");y.value=e.documents||[]}async function x(){const r=a.value.trim(),e=r?`/api/kbase/qa?topic=${encodeURIComponent(r)}`:"/api/kbase/qa",t=await fetch(e),j=await t.json();if(!t.ok)throw new Error(j.error||"加载问答失败");u.value=j.items||[]}async function T(){m.value=!0;try{await Promise.all([v(),x()])}catch(r){f.error(r.message||"加载失败")}finally{m.value=!1}}async function $(r){if(!l.value.trim()){f.warning("请输入文档路径");return}m.value=!0;try{const e=await fetch("/api/kbase/ingest",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({path:l.value.trim(),force:r})}),t=await e.json();if(!e.ok)throw new Error(t.error||"导入失败");f.success(t.skipped?"文档已存在，已跳过":`导入成功 (${t.chunks} chunks)`),await v()}catch(e){f.error(e.message||"导入失败")}finally{m.value=!1}}function B(r){const e=`/api/kbase/export/qa?format=markdown${r?"&by_topic=1":""}`;window.open(e,"_blank")}function W(){o.push({name:"Chat"})}function A(r){return r?new Date(r*1e3).toLocaleString():"-"}return te(T),(r,e)=>(i(),d("div",le,[n("header",de,[n("div",ue,[p(c(_),{quaternary:"",onClick:W},{default:h(()=>[...e[6]||(e[6]=[k(" ← 返回聊天 ",-1)])]),_:1}),e[7]||(e[7]=n("h1",{class:"text-lg font-medium"}," 知识库 ",-1))]),p(c(_),{loading:m.value,onClick:T},{default:h(()=>[...e[8]||(e[8]=[k(" 刷新 ",-1)])]),_:1},8,["loading"])]),p(c(re),{show:m.value,class:"flex-1 overflow-auto"},{default:h(()=>[n("div",ce,[n("section",null,[e[11]||(e[11]=n("h2",{class:"text-base font-semibold mb-3"}," 文档 ",-1)),n("div",pe,[p(c(I),{value:l.value,"onUpdate:value":e[0]||(e[0]=t=>l.value=t),placeholder:"文档路径",style:{width:"360px"}},null,8,["value"]),p(c(_),{type:"primary",onClick:e[1]||(e[1]=t=>$(!1))},{default:h(()=>[...e[9]||(e[9]=[k(" 导入 ",-1)])]),_:1}),p(c(_),{onClick:e[2]||(e[2]=t=>$(!0))},{default:h(()=>[...e[10]||(e[10]=[k(" 强制重建 ",-1)])]),_:1})]),y.value.length===0?(i(),d("div",fe," 暂无文档 ")):(i(),d("div",me,[(i(!0),d(q,null,N(y.value,t=>(i(),d("div",{key:t.id,class:"p-3 rounded border dark:border-neutral-700 text-sm"},[n("div",ve,g(t.title),1),n("div",ye,g(t.chunk_count)+" chunks · "+g(t.source_path||t.id),1)]))),128))]))]),n("section",null,[n("div",he,[e[14]||(e[14]=n("h2",{class:"text-base font-semibold"}," 历史问答 ",-1)),n("div",ge,[p(c(_),{size:"small",onClick:e[3]||(e[3]=t=>B(!1))},{default:h(()=>[...e[12]||(e[12]=[k(" 导出 Markdown ",-1)])]),_:1}),p(c(_),{size:"small",onClick:e[4]||(e[4]=t=>B(!0))},{default:h(()=>[...e[13]||(e[13]=[k(" 按主题导出 ",-1)])]),_:1})])]),n("div",xe,[p(c(I),{value:a.value,"onUpdate:value":e[5]||(e[5]=t=>a.value=t),placeholder:"按主题过滤",style:{width:"240px"}},null,8,["value"]),p(c(_),{onClick:x},{default:h(()=>[...e[15]||(e[15]=[k(" 筛选 ",-1)])]),_:1})]),n("div",ke,[(i(!0),d(q,null,N(b.value,t=>(i(),O(c(ne),{key:t.name,size:"small",bordered:!1},{default:h(()=>[k(g(t.name)+" ("+g(t.qa_count)+") ",1)]),_:2},1024))),128))]),u.value.length===0?(i(),d("div",be," 暂无问答，去聊天页提问后会自动沉淀 ")):(i(),d("div",we,[(i(!0),d(q,null,N(u.value,(t,j)=>(i(),d("div",{key:j,class:"p-4 rounded border dark:border-neutral-700"},[n("div",_e,g(t.question),1),n("div",Ce,g(t.topic_name||"未分类")+" · "+g(A(t.created_at)),1),n("pre",ze,g(t.answer),1)]))),128))]))])])]),_:1},8,["show"])]))}});export{$e as default};
