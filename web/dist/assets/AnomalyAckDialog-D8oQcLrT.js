import{E as e,Et as t,I as n,N as r,U as i,W as a,_ as o,at as s,d as c,et as l,f as u,l as d,n as f,p,u as m,v as h,y as g}from"./client-DnDgYBvu.js";import{_,f as v,r as y,t as b}from"./_plugin-vue_export-helper-CzDtOvb_.js";import{r as x}from"./index-QSx0ByDY.js";import{i as S,r as C}from"./inputtext-CnrsRCxJ.js";import{t as w}from"./textarea-Cx6KFByE.js";var T=v.extend({name:`toggleswitch`,style:`
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
`,classes:{root:function(e){var t=e.instance,n=e.props;return[`p-toggleswitch p-component`,{"p-toggleswitch-checked":t.checked,"p-disabled":n.disabled,"p-invalid":t.$invalid}]},input:`p-toggleswitch-input`,slider:`p-toggleswitch-slider`,handle:`p-toggleswitch-handle`},inlineStyles:{root:{position:`relative`}}}),E={name:`ToggleSwitch`,extends:{name:`BaseToggleSwitch`,extends:C,props:{trueValue:{type:null,default:!0},falseValue:{type:null,default:!1},readonly:{type:Boolean,default:!1},tabindex:{type:Number,default:null},inputId:{type:String,default:null},inputClass:{type:[String,Object],default:null},inputStyle:{type:Object,default:null},ariaLabelledby:{type:String,default:null},ariaLabel:{type:String,default:null}},style:T,provide:function(){return{$pcToggleSwitch:this,$parentInstance:this}}},inheritAttrs:!1,emits:[`change`,`focus`,`blur`],methods:{getPTOptions:function(e){return(e===`root`?this.ptmi:this.ptm)(e,{context:{checked:this.checked,disabled:this.disabled}})},onChange:function(e){if(!this.disabled&&!this.readonly){var t=this.checked?this.falseValue:this.trueValue;this.writeValue(t,e),this.$emit(`change`,e)}},onFocus:function(e){this.$emit(`focus`,e)},onBlur:function(e){var t,n;this.$emit(`blur`,e),(t=(n=this.formField).onBlur)==null||t.call(n,e)}},computed:{checked:function(){return this.d_value===this.trueValue},dataP:function(){return _({checked:this.checked,disabled:this.disabled,invalid:this.$invalid})}}},D=[`data-p-checked`,`data-p-disabled`,`data-p`],O=[`id`,`checked`,`tabindex`,`disabled`,`readonly`,`aria-checked`,`aria-labelledby`,`aria-label`,`aria-invalid`],k=[`data-p`],A=[`data-p`];function j(t,i,a,o,s,c){return r(),p(`div`,e({class:t.cx(`root`),style:t.sx(`root`)},c.getPTOptions(`root`),{"data-p-checked":c.checked,"data-p-disabled":t.disabled,"data-p":c.dataP}),[m(`input`,e({id:t.inputId,type:`checkbox`,role:`switch`,class:[t.cx(`input`),t.inputClass],style:t.inputStyle,checked:c.checked,tabindex:t.tabindex,disabled:t.disabled,readonly:t.readonly,"aria-checked":c.checked,"aria-labelledby":t.ariaLabelledby,"aria-label":t.ariaLabel,"aria-invalid":t.invalid||void 0,onFocus:i[0]||=function(){return c.onFocus&&c.onFocus.apply(c,arguments)},onBlur:i[1]||=function(){return c.onBlur&&c.onBlur.apply(c,arguments)},onChange:i[2]||=function(){return c.onChange&&c.onChange.apply(c,arguments)}},c.getPTOptions(`input`)),null,16,O),m(`div`,e({class:t.cx(`slider`)},c.getPTOptions(`slider`),{"data-p":c.dataP}),[m(`div`,e({class:t.cx(`handle`)},c.getPTOptions(`handle`),{"data-p":c.dataP}),[n(t.$slots,`handle`,{checked:c.checked})],16,A)],16,k)],16,D)}E.render=j;var M={key:0,class:`ack-topology`},N=b(g({__name:`AnomalyAckDialog`,props:{target:{}},emits:[`close`,`done`],setup(e,{emit:n}){let g=e,_=n,v=new Set([`orphan_idle`,`replaced`]),b=l(``),C=l(!1),T=l(!1),D=l(``);i(()=>g.target,()=>{b.value=``,C.value=!1,D.value=``});let O=d(()=>!!g.target&&!!g.target.inn&&v.has(g.target.kind));async function k(){let e=g.target;if(e&&b.value.trim()){T.value=!0;try{C.value&&O.value?await f.putTopology({inn:e.inn,kpp:e.kpp,edoId:e.edoId,purpose:b.value.trim()}):await f.acknowledge(e.edoId,e.fingerprint,b.value),_(`done`)}catch(e){D.value=e instanceof Error?e.message:`Не удалось сохранить пометку.`}finally{T.value=!1}}}return(n,i)=>(r(),c(s(x),{visible:e.target!==null,modal:``,header:`Скрыть находку как допустимую`,style:{width:`28rem`,maxWidth:`calc(100vw - 32px)`},"onUpdate:visible":i[3]||=e=>_(`close`)},{footer:a(()=>[h(s(y),{label:`Отмена`,text:``,onClick:i[2]||=e=>_(`close`)}),h(s(y),{label:`Пометить`,loading:T.value,disabled:!b.value.trim(),onClick:k},null,8,[`loading`,`disabled`])]),default:a(()=>[i[5]||=m(`p`,{class:`ack-hint`},` Пометка скроет именно это состояние. Если оно изменится — трафик пойдёт, сменится логин или владелец — находка появится снова. `,-1),h(s(w),{modelValue:b.value,"onUpdate:modelValue":i[0]||=e=>b.value=e,class:`ack-reason`,rows:`3`,"auto-resize":``,placeholder:`Причина, например: разные виды деятельности`},null,8,[`modelValue`]),O.value?(r(),p(`label`,M,[h(s(E),{modelValue:C.value,"onUpdate:modelValue":i[1]||=e=>C.value=e},null,8,[`modelValue`]),i[4]||=m(`span`,null,` Законная связь организации: запомнить в реестре топологии — гасит и будущие сигналы «без владельца» и «замена» по этой паре ИНН и идентификатора `,-1)])):u(``,!0),D.value?(r(),c(s(S),{key:1,severity:`error`,closable:!1},{default:a(()=>[o(t(D.value),1)]),_:1})):u(``,!0)]),_:1},8,[`visible`]))}}),[[`__scopeId`,`data-v-a4b930cc`]]);export{N as t};