import{$ as e,F as t,H as n,M as r,P as i,St as a,T as o,Tt as s,U as c,_ as l,d as u,f as d,g as f,it as p,l as m,n as h,o as g,p as _,u as v,v as y,wt as b}from"./client-JgoGY2Lg.js";import{i as x,n as S,pt as C,u as w,x as T}from"./index-swRijxKI.js";import{i as E,t as D}from"./message-eMYM0UmV.js";import{t as O}from"./textarea-CZ-d7j6Y.js";var k=T.extend({name:`toggleswitch`,style:`
    .p-toggleswitch {
        display: inline-block;
        width: dt('toggleswitch.width');
        height: dt('toggleswitch.height');
    }

    .p-toggleswitch-input {
        cursor: pointer;
        appearance: none;
        position: absolute;
        top: 0;
        inset-inline-start: 0;
        width: 100%;
        height: 100%;
        padding: 0;
        margin: 0;
        opacity: 0;
        z-index: 1;
        outline: 0 none;
        border-radius: dt('toggleswitch.border.radius');
    }

    .p-toggleswitch-slider {
        cursor: pointer;
        width: 100%;
        height: 100%;
        border-width: dt('toggleswitch.border.width');
        border-style: solid;
        border-color: dt('toggleswitch.border.color');
        background: dt('toggleswitch.background');
        transition:
            background dt('toggleswitch.transition.duration'),
            color dt('toggleswitch.transition.duration'),
            border-color dt('toggleswitch.transition.duration'),
            outline-color dt('toggleswitch.transition.duration'),
            box-shadow dt('toggleswitch.transition.duration');
        border-radius: dt('toggleswitch.border.radius');
        outline-color: transparent;
        box-shadow: dt('toggleswitch.shadow');
    }

    .p-toggleswitch-handle {
        position: absolute;
        top: 50%;
        display: flex;
        justify-content: center;
        align-items: center;
        background: dt('toggleswitch.handle.background');
        color: dt('toggleswitch.handle.color');
        width: dt('toggleswitch.handle.size');
        height: dt('toggleswitch.handle.size');
        inset-inline-start: dt('toggleswitch.gap');
        margin-block-start: calc(-1 * calc(dt('toggleswitch.handle.size') / 2));
        border-radius: dt('toggleswitch.handle.border.radius');
        transition:
            background dt('toggleswitch.transition.duration'),
            color dt('toggleswitch.transition.duration'),
            inset-inline-start dt('toggleswitch.slide.duration'),
            box-shadow dt('toggleswitch.slide.duration');
    }

    .p-toggleswitch.p-toggleswitch-checked .p-toggleswitch-slider {
        background: dt('toggleswitch.checked.background');
        border-color: dt('toggleswitch.checked.border.color');
    }

    .p-toggleswitch.p-toggleswitch-checked .p-toggleswitch-handle {
        background: dt('toggleswitch.handle.checked.background');
        color: dt('toggleswitch.handle.checked.color');
        inset-inline-start: calc(dt('toggleswitch.width') - calc(dt('toggleswitch.handle.size') + dt('toggleswitch.gap')));
    }

    .p-toggleswitch:not(.p-disabled):has(.p-toggleswitch-input:hover) .p-toggleswitch-slider {
        background: dt('toggleswitch.hover.background');
        border-color: dt('toggleswitch.hover.border.color');
    }

    .p-toggleswitch:not(.p-disabled):has(.p-toggleswitch-input:hover) .p-toggleswitch-handle {
        background: dt('toggleswitch.handle.hover.background');
        color: dt('toggleswitch.handle.hover.color');
    }

    .p-toggleswitch:not(.p-disabled):has(.p-toggleswitch-input:hover).p-toggleswitch-checked .p-toggleswitch-slider {
        background: dt('toggleswitch.checked.hover.background');
        border-color: dt('toggleswitch.checked.hover.border.color');
    }

    .p-toggleswitch:not(.p-disabled):has(.p-toggleswitch-input:hover).p-toggleswitch-checked .p-toggleswitch-handle {
        background: dt('toggleswitch.handle.checked.hover.background');
        color: dt('toggleswitch.handle.checked.hover.color');
    }

    .p-toggleswitch:not(.p-disabled):has(.p-toggleswitch-input:focus-visible) .p-toggleswitch-slider {
        box-shadow: dt('toggleswitch.focus.ring.shadow');
        outline: dt('toggleswitch.focus.ring.width') dt('toggleswitch.focus.ring.style') dt('toggleswitch.focus.ring.color');
        outline-offset: dt('toggleswitch.focus.ring.offset');
    }

    .p-toggleswitch.p-invalid > .p-toggleswitch-slider {
        border-color: dt('toggleswitch.invalid.border.color');
    }

    .p-toggleswitch.p-disabled {
        opacity: 1;
    }

    .p-toggleswitch.p-disabled .p-toggleswitch-slider {
        background: dt('toggleswitch.disabled.background');
    }

    .p-toggleswitch.p-disabled .p-toggleswitch-handle {
        background: dt('toggleswitch.handle.disabled.background');
    }
`,classes:{root:function(e){var t=e.instance,n=e.props;return[`p-toggleswitch p-component`,{"p-toggleswitch-checked":t.checked,"p-disabled":n.disabled,"p-invalid":t.$invalid}]},input:`p-toggleswitch-input`,slider:`p-toggleswitch-slider`,handle:`p-toggleswitch-handle`},inlineStyles:{root:{position:`relative`}}}),A={name:`ToggleSwitch`,extends:{name:`BaseToggleSwitch`,extends:E,props:{trueValue:{type:null,default:!0},falseValue:{type:null,default:!1},readonly:{type:Boolean,default:!1},tabindex:{type:Number,default:null},inputId:{type:String,default:null},inputClass:{type:[String,Object],default:null},inputStyle:{type:Object,default:null},ariaLabelledby:{type:String,default:null},ariaLabel:{type:String,default:null}},style:k,provide:function(){return{$pcToggleSwitch:this,$parentInstance:this}}},inheritAttrs:!1,emits:[`change`,`focus`,`blur`],methods:{getPTOptions:function(e){return(e===`root`?this.ptmi:this.ptm)(e,{context:{checked:this.checked,disabled:this.disabled}})},onChange:function(e){if(!this.disabled&&!this.readonly){var t=this.checked?this.falseValue:this.trueValue;this.writeValue(t,e),this.$emit(`change`,e)}},onFocus:function(e){this.$emit(`focus`,e)},onBlur:function(e){var t,n;this.$emit(`blur`,e),(t=(n=this.formField).onBlur)==null||t.call(n,e)}},computed:{checked:function(){return this.d_value===this.trueValue},dataP:function(){return C({checked:this.checked,disabled:this.disabled,invalid:this.$invalid})}}},j=[`data-p-checked`,`data-p-disabled`,`data-p`],M=[`id`,`checked`,`tabindex`,`disabled`,`readonly`,`aria-checked`,`aria-labelledby`,`aria-label`,`aria-invalid`],N=[`data-p`],P=[`data-p`];function F(e,n,i,a,s,c){return r(),_(`div`,o({class:e.cx(`root`),style:e.sx(`root`)},c.getPTOptions(`root`),{"data-p-checked":c.checked,"data-p-disabled":e.disabled,"data-p":c.dataP}),[v(`input`,o({id:e.inputId,type:`checkbox`,role:`switch`,class:[e.cx(`input`),e.inputClass],style:e.inputStyle,checked:c.checked,tabindex:e.tabindex,disabled:e.disabled,readonly:e.readonly,"aria-checked":c.checked,"aria-labelledby":e.ariaLabelledby,"aria-label":e.ariaLabel,"aria-invalid":e.invalid||void 0,onFocus:n[0]||=function(){return c.onFocus&&c.onFocus.apply(c,arguments)},onBlur:n[1]||=function(){return c.onBlur&&c.onBlur.apply(c,arguments)},onChange:n[2]||=function(){return c.onChange&&c.onChange.apply(c,arguments)}},c.getPTOptions(`input`)),null,16,M),v(`div`,o({class:e.cx(`slider`)},c.getPTOptions(`slider`),{"data-p":c.dataP}),[v(`div`,o({class:e.cx(`handle`)},c.getPTOptions(`handle`),{"data-p":c.dataP}),[t(e.$slots,`handle`,{checked:c.checked})],16,P)],16,N)],16,j)}A.render=F;var I={key:0,class:`ack-topology`},L=S(y({__name:`AnomalyAckDialog`,props:{target:{}},emits:[`close`,`done`],setup(t,{emit:i}){let a=t,o=i,g=new Set([`orphan_idle`,`replaced`]),y=e(``),b=e(!1),S=e(!1),C=e(``);n(()=>a.target,()=>{y.value=``,b.value=!1,C.value=``});let T=m(()=>!!a.target&&!!a.target.inn&&g.has(a.target.kind));async function E(){let e=a.target;if(e&&y.value.trim()){S.value=!0;try{b.value&&T.value?await h.putTopology({inn:e.inn,kpp:e.kpp,edoId:e.edoId,purpose:y.value.trim()}):await h.acknowledge(e.edoId,e.fingerprint,y.value),o(`done`)}catch(e){C.value=e instanceof Error?e.message:`Не удалось сохранить пометку.`}finally{S.value=!1}}}return(e,n)=>(r(),u(p(x),{visible:t.target!==null,modal:``,header:`Скрыть находку как допустимую`,style:{width:`28rem`,maxWidth:`calc(100vw - 32px)`},"onUpdate:visible":n[3]||=e=>o(`close`)},{footer:c(()=>[l(p(w),{label:`Отмена`,text:``,onClick:n[2]||=e=>o(`close`)}),l(p(w),{label:`Пометить`,loading:S.value,disabled:!y.value.trim(),onClick:E},null,8,[`loading`,`disabled`])]),default:c(()=>[n[5]||=v(`p`,{class:`ack-hint`},` Пометка скроет именно это состояние. Если оно изменится — трафик пойдёт, сменится логин или владелец — находка появится снова. `,-1),l(p(O),{modelValue:y.value,"onUpdate:modelValue":n[0]||=e=>y.value=e,class:`ack-reason`,rows:`3`,"auto-resize":``,placeholder:`Причина, например: разные виды деятельности`},null,8,[`modelValue`]),T.value?(r(),_(`label`,I,[l(p(A),{modelValue:b.value,"onUpdate:modelValue":n[1]||=e=>b.value=e},null,8,[`modelValue`]),n[4]||=v(`span`,null,` Законная связь организации: запомнить в реестре топологии — гасит и будущие сигналы «без владельца» и «замена» по этой паре ИНН и идентификатора `,-1)])):d(``,!0),C.value?(r(),u(p(D),{key:1,severity:`error`,closable:!1},{default:c(()=>[f(s(C.value),1)]),_:1})):d(``,!0)]),_:1},8,[`visible`]))}}),[[`__scopeId`,`data-v-a4b930cc`]]),R={class:`month-bars`},z=[`aria-label`],B={class:`area`},V=[`aria-label`],H={key:2,class:`mark missing`},U={class:`tip`},W={class:`axis`},G={class:`year`},K={class:`as-table`},q=S(y({__name:`MonthBars`,props:{bars:{},limit:{},unit:{},title:{}},setup(e){let t=e,n=m(()=>Math.max(1,t.limit??0,...t.bars.map(e=>e.value??0))),o=m(()=>{let e=t.bars.filter(e=>e.value!==null),n=e.at(-1),r=e.reduce((e,t)=>e===void 0||(t.value??0)>(e.value??0)?t:e,void 0);return new Set([n?.key,r?.key])});function c(e){return`${(e??0)/n.value*100}%`}function l(e){let n=e.value===null?`нет данных`:`${e.value} ${t.unit}`;return[e.label,n,e.note].filter(Boolean).join(` · `)}function u(e){let t=e.lastIndexOf(` `);return t>0?e.slice(0,t):e}function p(e){let t=e.lastIndexOf(` `);return t>0?e.slice(t):``}return(t,n)=>(r(),_(`figure`,R,[v(`div`,{class:`plot`,role:`img`,"aria-label":e.title},[v(`div`,B,[e.limit?(r(),_(`div`,{key:0,class:`limit`,style:b({bottom:c(e.limit)})},[v(`span`,null,`лимит `+s(e.limit),1)],4)):d(``,!0),(r(!0),_(g,null,i(e.bars,e=>(r(),_(`div`,{key:e.key,class:`month-col`,tabindex:`0`,"aria-label":l(e)},[e.value!==null&&o.value.has(e.key)?(r(),_(`span`,{key:0,class:`value`,style:b({bottom:c(e.value)})},s(e.value),5)):d(``,!0),e.value===null?(r(),_(`div`,H)):(r(),_(`div`,{key:1,class:a([`mark`,{over:e.over}]),style:b({height:c(e.value)})},null,6)),v(`span`,U,s(l(e)),1)],8,V))),128))])],8,z),v(`div`,W,[(r(!0),_(g,null,i(e.bars,e=>(r(),_(`span`,{key:e.key},[f(s(u(e.label)),1),v(`span`,G,s(p(e.label)),1)]))),128))]),v(`details`,K,[n[0]||=v(`summary`,null,`Таблицей`,-1),v(`table`,null,[v(`tbody`,null,[(r(!0),_(g,null,i(e.bars,e=>(r(),_(`tr`,{key:e.key},[v(`th`,null,s(e.label),1),v(`td`,null,s(e.value??`—`),1),v(`td`,null,s(e.note),1)]))),128))])])])]))}}),[[`__scopeId`,`data-v-2a4cd37d`]]);export{L as n,q as t};