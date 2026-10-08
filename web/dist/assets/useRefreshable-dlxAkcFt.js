import{$ as e,A as t,F as n,H as r,M as i,R as a,T as o,Tt as s,d as c,f as l,l as u,p as d,u as f}from"./client-JgoGY2Lg.js";import{_ as p,g as m,h,pt as g,x as _}from"./index-swRijxKI.js";import{h as v}from"./domain-CdZZIA2H.js";var y={name:`ChevronRightIcon`,extends:h};function b(e){return w(e)||C(e)||S(e)||x()}function x(){throw TypeError(`Invalid attempt to spread non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`)}function S(e,t){if(e){if(typeof e==`string`)return T(e,t);var n={}.toString.call(e).slice(8,-1);return n===`Object`&&e.constructor&&(n=e.constructor.name),n===`Map`||n===`Set`?Array.from(e):n===`Arguments`||/^(?:Ui|I)nt(?:8|16|32)(?:Clamped)?Array$/.test(n)?T(e,t):void 0}}function C(e){if(typeof Symbol<`u`&&e[Symbol.iterator]!=null||e[`@@iterator`]!=null)return Array.from(e)}function w(e){if(Array.isArray(e))return T(e)}function T(e,t){(t==null||t>e.length)&&(t=e.length);for(var n=0,r=Array(t);n<t;n++)r[n]=e[n];return r}function E(e,t,n,r,a,s){return i(),d(`svg`,o({width:`14`,height:`14`,viewBox:`0 0 14 14`,fill:`none`,xmlns:`http://www.w3.org/2000/svg`},e.pti()),b(t[0]||=[f(`path`,{d:`M4.38708 13C4.28408 13.0005 4.18203 12.9804 4.08691 12.9409C3.99178 12.9014 3.9055 12.8433 3.83313 12.7701C3.68634 12.6231 3.60388 12.4238 3.60388 12.2161C3.60388 12.0084 3.68634 11.8091 3.83313 11.6622L8.50507 6.99022L3.83313 2.31827C3.69467 2.16968 3.61928 1.97313 3.62287 1.77005C3.62645 1.56698 3.70872 1.37322 3.85234 1.22959C3.99596 1.08597 4.18972 1.00371 4.3928 1.00012C4.59588 0.996539 4.79242 1.07192 4.94102 1.21039L10.1669 6.43628C10.3137 6.58325 10.3962 6.78249 10.3962 6.99022C10.3962 7.19795 10.3137 7.39718 10.1669 7.54416L4.94102 12.7701C4.86865 12.8433 4.78237 12.9014 4.68724 12.9409C4.59212 12.9804 4.49007 13.0005 4.38708 13Z`,fill:`currentColor`},null,-1)]),16)}y.render=E;var D=_.extend({name:`tag`,style:`
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
`,classes:{root:function(e){var t=e.props;return[`p-tag p-component`,{"p-tag-info":t.severity===`info`,"p-tag-success":t.severity===`success`,"p-tag-warn":t.severity===`warn`,"p-tag-danger":t.severity===`danger`,"p-tag-secondary":t.severity===`secondary`,"p-tag-contrast":t.severity===`contrast`,"p-tag-rounded":t.rounded}]},icon:`p-tag-icon`,label:`p-tag-label`}}),O={name:`BaseTag`,extends:m,props:{value:null,severity:null,rounded:Boolean,icon:String},style:D,provide:function(){return{$pcTag:this,$parentInstance:this}}};function k(e){"@babel/helpers - typeof";return k=typeof Symbol==`function`&&typeof Symbol.iterator==`symbol`?function(e){return typeof e}:function(e){return e&&typeof Symbol==`function`&&e.constructor===Symbol&&e!==Symbol.prototype?`symbol`:typeof e},k(e)}function A(e,t,n){return(t=j(t))in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function j(e){var t=M(e,`string`);return k(t)==`symbol`?t:t+``}function M(e,t){if(k(e)!=`object`||!e)return e;var n=e[Symbol.toPrimitive];if(n!==void 0){var r=n.call(e,t);if(k(r)!=`object`)return r;throw TypeError(`@@toPrimitive must return a primitive value.`)}return(t===`string`?String:Number)(e)}var N={name:`Tag`,extends:O,inheritAttrs:!1,computed:{dataP:function(){return g(A({rounded:this.rounded},this.severity,this.severity))}}},P=[`data-p`];function F(e,t,r,u,p,m){return i(),d(`span`,o({class:e.cx(`root`),"data-p":m.dataP},e.ptmi(`root`)),[e.$slots.icon?(i(),c(a(e.$slots.icon),o({key:0,class:e.cx(`icon`)},e.ptm(`icon`)),null,16,[`class`])):e.icon?(i(),d(`span`,o({key:1,class:[e.cx(`icon`),e.icon]},e.ptm(`icon`)),null,16)):l(``,!0),e.value!=null||e.$slots.default?n(e.$slots,`default`,{key:2},function(){return[f(`span`,o({class:e.cx(`label`)},e.ptm(`label`)),s(e.value),17)]}):l(``,!0)],16,P)}N.render=F;function I(n,i){let a=p(),o=e(!0),s=e(!1),c=e(``),l=e(!1);async function d(e=!1){e?s.value=!0:o.value=!0;try{await n(),c.value=``,l.value=!1,a.markRefreshed()}catch(t){let n=t instanceof Error?t.message:i.failText;e?l.value=!0:c.value=n}finally{o.value=!1,s.value=!1}}let f=u(()=>v(a.lastRefreshAt));return i.auto!==!1&&(t(()=>d()),r(()=>a.lastEvent,()=>d(!0))),{loading:o,refreshing:s,error:c,refreshFailed:l,dataAsOf:f,load:d}}export{N as n,y as r,I as t};