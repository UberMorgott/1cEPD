import{$ as e,F as t,H as n,M as r,T as i,Tt as a,U as o,_ as s,d as c,f as l,g as u,it as d,l as f,n as p,p as m,u as h,v as g}from"./client-D_s8yrfH.js";import{d as _,r as v,t as y,tt as b}from"./_plugin-vue_export-helper-BUaoTuDi.js";import{r as x}from"./index-Cy_YKy_V.js";import{i as S,r as C}from"./inputtext-IavC6RRl.js";import{t as w}from"./textarea-BP-I5YiV.js";var T=_.extend({name:`toggleswitch`,style:`
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
`,classes:{root:function(e){var t=e.instance,n=e.props;return[`p-toggleswitch p-component`,{"p-toggleswitch-checked":t.checked,"p-disabled":n.disabled,"p-invalid":t.$invalid}]},input:`p-toggleswitch-input`,slider:`p-toggleswitch-slider`,handle:`p-toggleswitch-handle`},inlineStyles:{root:{position:`relative`}}}),E={name:`ToggleSwitch`,extends:{name:`BaseToggleSwitch`,extends:C,props:{trueValue:{type:null,default:!0},falseValue:{type:null,default:!1},readonly:{type:Boolean,default:!1},tabindex:{type:Number,default:null},inputId:{type:String,default:null},inputClass:{type:[String,Object],default:null},inputStyle:{type:Object,default:null},ariaLabelledby:{type:String,default:null},ariaLabel:{type:String,default:null}},style:T,provide:function(){return{$pcToggleSwitch:this,$parentInstance:this}}},inheritAttrs:!1,emits:[`change`,`focus`,`blur`],methods:{getPTOptions:function(e){return(e===`root`?this.ptmi:this.ptm)(e,{context:{checked:this.checked,disabled:this.disabled}})},onChange:function(e){if(!this.disabled&&!this.readonly){var t=this.checked?this.falseValue:this.trueValue;this.writeValue(t,e),this.$emit(`change`,e)}},onFocus:function(e){this.$emit(`focus`,e)},onBlur:function(e){var t,n;this.$emit(`blur`,e),(t=(n=this.formField).onBlur)==null||t.call(n,e)}},computed:{checked:function(){return this.d_value===this.trueValue},dataP:function(){return b({checked:this.checked,disabled:this.disabled,invalid:this.$invalid})}}},D=[`data-p-checked`,`data-p-disabled`,`data-p`],O=[`id`,`checked`,`tabindex`,`disabled`,`readonly`,`aria-checked`,`aria-labelledby`,`aria-label`,`aria-invalid`],k=[`data-p`],A=[`data-p`];function j(e,n,a,o,s,c){return r(),m(`div`,i({class:e.cx(`root`),style:e.sx(`root`)},c.getPTOptions(`root`),{"data-p-checked":c.checked,"data-p-disabled":e.disabled,"data-p":c.dataP}),[h(`input`,i({id:e.inputId,type:`checkbox`,role:`switch`,class:[e.cx(`input`),e.inputClass],style:e.inputStyle,checked:c.checked,tabindex:e.tabindex,disabled:e.disabled,readonly:e.readonly,"aria-checked":c.checked,"aria-labelledby":e.ariaLabelledby,"aria-label":e.ariaLabel,"aria-invalid":e.invalid||void 0,onFocus:n[0]||=function(){return c.onFocus&&c.onFocus.apply(c,arguments)},onBlur:n[1]||=function(){return c.onBlur&&c.onBlur.apply(c,arguments)},onChange:n[2]||=function(){return c.onChange&&c.onChange.apply(c,arguments)}},c.getPTOptions(`input`)),null,16,O),h(`div`,i({class:e.cx(`slider`)},c.getPTOptions(`slider`),{"data-p":c.dataP}),[h(`div`,i({class:e.cx(`handle`)},c.getPTOptions(`handle`),{"data-p":c.dataP}),[t(e.$slots,`handle`,{checked:c.checked})],16,A)],16,k)],16,D)}E.render=j;var M={key:0,class:`ack-topology`},N=y(g({__name:`AnomalyAckDialog`,props:{target:{}},emits:[`close`,`done`],setup(t,{emit:i}){let g=t,_=i,y=new Set([`orphan_idle`,`replaced`]),b=e(``),C=e(!1),T=e(!1),D=e(``);n(()=>g.target,()=>{b.value=``,C.value=!1,D.value=``});let O=f(()=>!!g.target&&!!g.target.inn&&y.has(g.target.kind));async function k(){let e=g.target;if(e&&b.value.trim()){T.value=!0;try{C.value&&O.value?await p.putTopology({inn:e.inn,kpp:e.kpp,edoId:e.edoId,purpose:b.value.trim()}):await p.acknowledge(e.edoId,e.fingerprint,b.value),_(`done`)}catch(e){D.value=e instanceof Error?e.message:`Не удалось сохранить пометку.`}finally{T.value=!1}}}return(e,n)=>(r(),c(d(x),{visible:t.target!==null,modal:``,header:`Скрыть находку как допустимую`,style:{width:`28rem`,maxWidth:`calc(100vw - 32px)`},"onUpdate:visible":n[3]||=e=>_(`close`)},{footer:o(()=>[s(d(v),{label:`Отмена`,text:``,onClick:n[2]||=e=>_(`close`)}),s(d(v),{label:`Пометить`,loading:T.value,disabled:!b.value.trim(),onClick:k},null,8,[`loading`,`disabled`])]),default:o(()=>[n[5]||=h(`p`,{class:`ack-hint`},` Пометка скроет именно это состояние. Если оно изменится — трафик пойдёт, сменится логин или владелец — находка появится снова. `,-1),s(d(w),{modelValue:b.value,"onUpdate:modelValue":n[0]||=e=>b.value=e,class:`ack-reason`,rows:`3`,"auto-resize":``,placeholder:`Причина, например: разные виды деятельности`},null,8,[`modelValue`]),O.value?(r(),m(`label`,M,[s(d(E),{modelValue:C.value,"onUpdate:modelValue":n[1]||=e=>C.value=e},null,8,[`modelValue`]),n[4]||=h(`span`,null,` Законная связь организации: запомнить в реестре топологии — гасит и будущие сигналы «без владельца» и «замена» по этой паре ИНН и идентификатора `,-1)])):l(``,!0),D.value?(r(),c(d(S),{key:1,severity:`error`,closable:!1},{default:o(()=>[u(a(D.value),1)]),_:1})):l(``,!0)]),_:1},8,[`visible`]))}}),[[`__scopeId`,`data-v-a4b930cc`]]);export{N as t};