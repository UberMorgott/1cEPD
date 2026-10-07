import{$ as e,A as t,F as n,H as r,M as i,R as a,T as o,Tt as s,d as c,f as l,l as u,p as d,u as f}from"./client-C0C9m9Fi.js";import{d as p,l as m,st as h}from"./_plugin-vue_export-helper-Dx1ydjeN.js";import{o as g}from"./index-CFTz59NC.js";import{g as _}from"./PageHeader-BvIrA8Yl.js";var v=p.extend({name:`tag`,style:`
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
`,classes:{root:function(e){var t=e.props;return[`p-tag p-component`,{"p-tag-info":t.severity===`info`,"p-tag-success":t.severity===`success`,"p-tag-warn":t.severity===`warn`,"p-tag-danger":t.severity===`danger`,"p-tag-secondary":t.severity===`secondary`,"p-tag-contrast":t.severity===`contrast`,"p-tag-rounded":t.rounded}]},icon:`p-tag-icon`,label:`p-tag-label`}}),y={name:`BaseTag`,extends:m,props:{value:null,severity:null,rounded:Boolean,icon:String},style:v,provide:function(){return{$pcTag:this,$parentInstance:this}}};function b(e){"@babel/helpers - typeof";return b=typeof Symbol==`function`&&typeof Symbol.iterator==`symbol`?function(e){return typeof e}:function(e){return e&&typeof Symbol==`function`&&e.constructor===Symbol&&e!==Symbol.prototype?`symbol`:typeof e},b(e)}function x(e,t,n){return(t=S(t))in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function S(e){var t=C(e,`string`);return b(t)==`symbol`?t:t+``}function C(e,t){if(b(e)!=`object`||!e)return e;var n=e[Symbol.toPrimitive];if(n!==void 0){var r=n.call(e,t);if(b(r)!=`object`)return r;throw TypeError(`@@toPrimitive must return a primitive value.`)}return(t===`string`?String:Number)(e)}var w={name:`Tag`,extends:y,inheritAttrs:!1,computed:{dataP:function(){return h(x({rounded:this.rounded},this.severity,this.severity))}}},T=[`data-p`];function E(e,t,r,u,p,m){return i(),d(`span`,o({class:e.cx(`root`),"data-p":m.dataP},e.ptmi(`root`)),[e.$slots.icon?(i(),c(a(e.$slots.icon),o({key:0,class:e.cx(`icon`)},e.ptm(`icon`)),null,16,[`class`])):e.icon?(i(),d(`span`,o({key:1,class:[e.cx(`icon`),e.icon]},e.ptm(`icon`)),null,16)):l(``,!0),e.value!=null||e.$slots.default?n(e.$slots,`default`,{key:2},function(){return[f(`span`,o({class:e.cx(`label`)},e.ptm(`label`)),s(e.value),17)]}):l(``,!0)],16,T)}w.render=E;function D(n,i){let a=g(),o=e(!0),s=e(!1),c=e(``),l=e(!1);async function d(e=!1){e?s.value=!0:o.value=!0;try{await n(),c.value=``,l.value=!1,a.markRefreshed()}catch(t){let n=t instanceof Error?t.message:i.failText;e?l.value=!0:c.value=n}finally{o.value=!1,s.value=!1}}let f=u(()=>_(a.lastRefreshAt));return i.auto!==!1&&(t(()=>d()),r(()=>a.lastEvent,()=>d(!0))),{loading:o,refreshing:s,error:c,refreshFailed:l,dataAsOf:f,load:d}}export{w as n,D as t};