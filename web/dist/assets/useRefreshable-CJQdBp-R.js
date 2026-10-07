import{E as e,Et as t,I as n,N as r,U as i,d as a,et as o,f as s,j as c,l,p as u,u as d,z as f}from"./client-BZwcUfJs.js";import{d as p,l as m,tt as h}from"./_plugin-vue_export-helper-D0JjzNkr.js";import{o as g}from"./index-YP-M_90f.js";import{g as _}from"./PageHeader-CkG0Tvk8.js";var v=p.extend({name:`tag`,style:`
    .p-tag {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        background: dt('tag.primary.background');
        color: dt('tag.primary.color');
        font-size: dt('tag.font.size');
        font-weight: dt('tag.font.weight');
        padding: dt('tag.padding');
        border-radius: dt('tag.border.radius');
        gap: dt('tag.gap');
    }

    .p-tag-icon {
        font-size: dt('tag.icon.size');
        width: dt('tag.icon.size');
        height: dt('tag.icon.size');
    }

    .p-tag-rounded {
        border-radius: dt('tag.rounded.border.radius');
    }

    .p-tag-success {
        background: dt('tag.success.background');
        color: dt('tag.success.color');
    }

    .p-tag-info {
        background: dt('tag.info.background');
        color: dt('tag.info.color');
    }

    .p-tag-warn {
        background: dt('tag.warn.background');
        color: dt('tag.warn.color');
    }

    .p-tag-danger {
        background: dt('tag.danger.background');
        color: dt('tag.danger.color');
    }

    .p-tag-secondary {
        background: dt('tag.secondary.background');
        color: dt('tag.secondary.color');
    }

    .p-tag-contrast {
        background: dt('tag.contrast.background');
        color: dt('tag.contrast.color');
    }
`,classes:{root:function(e){var t=e.props;return[`p-tag p-component`,{"p-tag-info":t.severity===`info`,"p-tag-success":t.severity===`success`,"p-tag-warn":t.severity===`warn`,"p-tag-danger":t.severity===`danger`,"p-tag-secondary":t.severity===`secondary`,"p-tag-contrast":t.severity===`contrast`,"p-tag-rounded":t.rounded}]},icon:`p-tag-icon`,label:`p-tag-label`}}),y={name:`BaseTag`,extends:m,props:{value:null,severity:null,rounded:Boolean,icon:String},style:v,provide:function(){return{$pcTag:this,$parentInstance:this}}};function b(e){"@babel/helpers - typeof";return b=typeof Symbol==`function`&&typeof Symbol.iterator==`symbol`?function(e){return typeof e}:function(e){return e&&typeof Symbol==`function`&&e.constructor===Symbol&&e!==Symbol.prototype?`symbol`:typeof e},b(e)}function x(e,t,n){return(t=S(t))in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function S(e){var t=C(e,`string`);return b(t)==`symbol`?t:t+``}function C(e,t){if(b(e)!=`object`||!e)return e;var n=e[Symbol.toPrimitive];if(n!==void 0){var r=n.call(e,t);if(b(r)!=`object`)return r;throw TypeError(`@@toPrimitive must return a primitive value.`)}return(t===`string`?String:Number)(e)}var w={name:`Tag`,extends:y,inheritAttrs:!1,computed:{dataP:function(){return h(x({rounded:this.rounded},this.severity,this.severity))}}},T=[`data-p`];function E(i,o,c,l,p,m){return r(),u(`span`,e({class:i.cx(`root`),"data-p":m.dataP},i.ptmi(`root`)),[i.$slots.icon?(r(),a(f(i.$slots.icon),e({key:0,class:i.cx(`icon`)},i.ptm(`icon`)),null,16,[`class`])):i.icon?(r(),u(`span`,e({key:1,class:[i.cx(`icon`),i.icon]},i.ptm(`icon`)),null,16)):s(``,!0),i.value!=null||i.$slots.default?n(i.$slots,`default`,{key:2},function(){return[d(`span`,e({class:i.cx(`label`)},i.ptm(`label`)),t(i.value),17)]}):s(``,!0)],16,T)}w.render=E;function D(e,t){let n=g(),r=o(!0),a=o(!1),s=o(``),u=o(!1);async function d(i=!1){i?a.value=!0:r.value=!0;try{await e(),s.value=``,u.value=!1,n.markRefreshed()}catch(e){let n=e instanceof Error?e.message:t.failText;i?u.value=!0:s.value=n}finally{r.value=!1,a.value=!1}}let f=l(()=>_(n.lastRefreshAt));return t.auto!==!1&&(c(()=>d()),i(()=>n.lastEvent,()=>d(!0))),{loading:r,refreshing:a,error:s,refreshFailed:u,dataAsOf:f,load:d}}export{w as n,D as t};