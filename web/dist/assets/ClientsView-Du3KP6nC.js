import{$ as e,A as t,F as n,H as r,I as i,L as a,M as o,P as s,R as c,St as l,T as u,Tt as d,U as f,W as p,_ as m,d as h,f as g,g as _,h as v,it as y,l as b,n as x,o as S,p as C,u as w,v as T}from"./client-Bfd28sF8.js";import{g as ee,h as te}from"./useApi-BPuI6ZR9-BCX5Khmn.js";import{$ as E,Ct as D,Dt as O,Et as k,G as ne,Q as re,T as A,X as j,_ as ie,a as M,c as ae,d as N,ft as P,g as F,i as oe,j as I,jt as se,l as ce,n as le,o as ue,pt as L,rt as de,s as R,u as z,ut as fe,v as pe,wt as B,x as V,xt as me,y as he}from"./index-BN9JGpiK.js";import{i as ge,t as H}from"./message-tFNVWxRU.js";import{n as _e,t as ve}from"./useRefreshable-BgKwbhYe.js";import{n as U,t as ye}from"./MonthBars-BNRDICXC.js";import{T as be,_ as xe,b as Se,c as W,d as Ce,g as we,i as Te,m as Ee,n as De,o as Oe,p as ke,r as Ae,t as G,u as je,v as K,x as Me,y as q}from"./domain-CdZZIA2H.js";import{i as J,n as Ne,r as Pe,t as Fe}from"./useInfiniteRows-DPeQi0vL.js";import{n as Ie,t as Le}from"./PageHeader-CTo3W_Op.js";import{n as Re,t as ze}from"./overlayeventbus-Dr8ChNzf.js";import{t as Be}from"./select-0NxrSUZx.js";import{t as Ve}from"./tooltip-W7zrda5R.js";var He=V.extend({name:`drawer`,style:`
    .p-drawer {
        display: flex;
        flex-direction: column;
        transform: translate3d(0px, 0px, 0px);
        position: relative;
        transition: transform 0.3s;
        background: dt('drawer.background');
        color: dt('drawer.color');
        border-style: solid;
        border-color: dt('drawer.border.color');
        box-shadow: dt('drawer.shadow');
    }

    .p-drawer-content {
        overflow-y: auto;
        flex-grow: 1;
        padding: dt('drawer.content.padding');
    }

    .p-drawer-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-shrink: 0;
        padding: dt('drawer.header.padding');
    }

    .p-drawer-footer {
        padding: dt('drawer.footer.padding');
    }

    .p-drawer-title {
        font-weight: dt('drawer.title.font.weight');
        font-size: dt('drawer.title.font.size');
    }

    .p-drawer-full .p-drawer {
        transition: none;
        transform: none;
        width: 100vw !important;
        height: 100vh !important;
        max-height: 100%;
        top: 0px !important;
        left: 0px !important;
        border-width: 1px;
    }

    .p-drawer-left .p-drawer-enter-active {
        animation: p-animate-drawer-enter-left 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }
    .p-drawer-left .p-drawer-leave-active {
        animation: p-animate-drawer-leave-left 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .p-drawer-right .p-drawer-enter-active {
        animation: p-animate-drawer-enter-right 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }
    .p-drawer-right .p-drawer-leave-active {
        animation: p-animate-drawer-leave-right 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .p-drawer-top .p-drawer-enter-active {
        animation: p-animate-drawer-enter-top 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }
    .p-drawer-top .p-drawer-leave-active {
        animation: p-animate-drawer-leave-top 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .p-drawer-bottom .p-drawer-enter-active {
        animation: p-animate-drawer-enter-bottom 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }
    .p-drawer-bottom .p-drawer-leave-active {
        animation: p-animate-drawer-leave-bottom 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .p-drawer-full .p-drawer-enter-active {
        animation: p-animate-drawer-enter-full 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }
    .p-drawer-full .p-drawer-leave-active {
        animation: p-animate-drawer-leave-full 0.5s cubic-bezier(0.32, 0.72, 0, 1);
    }
    
    .p-drawer-left .p-drawer {
        width: 20rem;
        height: 100%;
        border-inline-end-width: 1px;
    }

    .p-drawer-right .p-drawer {
        width: 20rem;
        height: 100%;
        border-inline-start-width: 1px;
    }

    .p-drawer-top .p-drawer {
        height: 10rem;
        width: 100%;
        border-block-end-width: 1px;
    }

    .p-drawer-bottom .p-drawer {
        height: 10rem;
        width: 100%;
        border-block-start-width: 1px;
    }

    .p-drawer-left .p-drawer-content,
    .p-drawer-right .p-drawer-content,
    .p-drawer-top .p-drawer-content,
    .p-drawer-bottom .p-drawer-content {
        width: 100%;
        height: 100%;
    }

    .p-drawer-open {
        display: flex;
    }

    .p-drawer-mask:dir(rtl) {
        flex-direction: row-reverse;
    }

    @keyframes p-animate-drawer-enter-left {
        from {
            transform: translate3d(-100%, 0px, 0px);
        }
    }

    @keyframes p-animate-drawer-leave-left {
        to {
            transform: translate3d(-100%, 0px, 0px);
        }
    }

    @keyframes p-animate-drawer-enter-right {
        from {
            transform: translate3d(100%, 0px, 0px);
        }
    }

    @keyframes p-animate-drawer-leave-right {
        to {
            transform: translate3d(100%, 0px, 0px);
        }
    }

    @keyframes p-animate-drawer-enter-top {
        from {
            transform: translate3d(0px, -100%, 0px);
        }
    }

    @keyframes p-animate-drawer-leave-top {
        to {
            transform: translate3d(0px, -100%, 0px);
        }
    }

    @keyframes p-animate-drawer-enter-bottom {
        from {
            transform: translate3d(0px, 100%, 0px);
        }
    }

    @keyframes p-animate-drawer-leave-bottom {
        to {
            transform: translate3d(0px, 100%, 0px);
        }
    }

    @keyframes p-animate-drawer-enter-full {
        from {
            opacity: 0;
            transform: scale(0.93);
        }
    }

    @keyframes p-animate-drawer-leave-full {
        to {
            opacity: 0;
            transform: scale(0.93);
        }
    }
`,classes:{mask:function(e){var t=e.instance,n=e.props,r=[`left`,`right`,`top`,`bottom`].find(function(e){return e===n.position});return[`p-drawer-mask`,{"p-overlay-mask p-overlay-mask-enter-active":n.modal,"p-drawer-open":t.containerVisible,"p-drawer-full":t.fullScreen},r?`p-drawer-${r}`:``]},root:function(e){return[`p-drawer p-component`,{"p-drawer-full":e.instance.fullScreen}]},header:`p-drawer-header`,title:`p-drawer-title`,pcCloseButton:`p-drawer-close-button`,content:`p-drawer-content`,footer:`p-drawer-footer`},inlineStyles:{mask:function(e){var t=e.position,n=e.modal;return{position:`fixed`,height:`100%`,width:`100%`,left:0,top:0,display:`flex`,justifyContent:t===`left`?`flex-start`:t===`right`?`flex-end`:`center`,alignItems:t===`top`?`flex-start`:t===`bottom`?`flex-end`:`center`,pointerEvents:n?`auto`:`none`}},root:{pointerEvents:`auto`}}}),Ue={name:`BaseDrawer`,extends:F,props:{visible:{type:Boolean,default:!1},position:{type:String,default:`left`},header:{type:null,default:null},baseZIndex:{type:Number,default:0},autoZIndex:{type:Boolean,default:!0},dismissable:{type:Boolean,default:!0},showCloseIcon:{type:Boolean,default:!0},closeButtonProps:{type:Object,default:function(){return{severity:`secondary`,text:!0,rounded:!0}}},closeIcon:{type:String,default:void 0},modal:{type:Boolean,default:!0},blockScroll:{type:Boolean,default:!1},closeOnEscape:{type:Boolean,default:!0}},style:He,provide:function(){return{$pcDrawer:this,$parentInstance:this}}};function Y(e){"@babel/helpers - typeof";return Y=typeof Symbol==`function`&&typeof Symbol.iterator==`symbol`?function(e){return typeof e}:function(e){return e&&typeof Symbol==`function`&&e.constructor===Symbol&&e!==Symbol.prototype?`symbol`:typeof e},Y(e)}function We(e,t,n){return(t=Ge(t))in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function Ge(e){var t=Ke(e,`string`);return Y(t)==`symbol`?t:t+``}function Ke(e,t){if(Y(e)!=`object`||!e)return e;var n=e[Symbol.toPrimitive];if(n!==void 0){var r=n.call(e,t);if(Y(r)!=`object`)return r;throw TypeError(`@@toPrimitive must return a primitive value.`)}return(t===`string`?String:Number)(e)}var qe={name:`Drawer`,extends:Ue,inheritAttrs:!1,emits:[`update:visible`,`show`,`after-show`,`hide`,`after-hide`,`before-hide`],data:function(){return{containerVisible:this.visible}},container:null,mask:null,content:null,headerContainer:null,footerContainer:null,closeButton:null,outsideClickListener:null,documentKeydownListener:null,watch:{dismissable:function(e){e&&!this.modal?this.bindOutsideClickListener():this.unbindOutsideClickListener()}},updated:function(){this.visible&&(this.containerVisible=this.visible)},beforeUnmount:function(){this.disableDocumentSettings(),this.mask&&this.autoZIndex&&A.clear(this.mask),this.container=null,this.mask=null},methods:{hide:function(){this.$emit(`update:visible`,!1)},onEnter:function(){this.$emit(`show`),this.focus(),this.bindDocumentKeyDownListener(),this.autoZIndex&&A.set(`modal`,this.mask,this.baseZIndex||this.$primevue.config.zIndex.modal)},onAfterEnter:function(){this.enableDocumentSettings(),this.$emit(`after-show`)},onBeforeLeave:function(){this.modal&&!this.isUnstyled&&j(this.mask,`p-overlay-mask-leave-active`),this.$emit(`before-hide`)},onLeave:function(){this.$emit(`hide`)},onAfterLeave:function(){this.autoZIndex&&A.clear(this.mask),this.unbindDocumentKeyDownListener(),this.containerVisible=!1,this.disableDocumentSettings(),this.$emit(`after-hide`)},onMaskClick:function(e){this.dismissable&&this.modal&&this.mask===e.target&&this.hide()},focus:function(){var e=function(e){return e&&e.querySelector(`[autofocus]`)},t=this.$slots.header&&e(this.headerContainer);t||(t=this.$slots.default&&e(this.container),t||(t=this.$slots.footer&&e(this.footerContainer),t||=this.closeButton)),t&&de(t)},enableDocumentSettings:function(){this.dismissable&&!this.modal&&this.bindOutsideClickListener(),this.blockScroll&&M()},disableDocumentSettings:function(){this.unbindOutsideClickListener(),this.blockScroll&&ue()},onKeydown:function(e){e.code===`Escape`&&this.closeOnEscape&&this.hide()},containerRef:function(e){this.container=e},maskRef:function(e){this.mask=e},contentRef:function(e){this.content=e},headerContainerRef:function(e){this.headerContainer=e},footerContainerRef:function(e){this.footerContainer=e},closeButtonRef:function(e){this.closeButton=e?e.$el:void 0},bindDocumentKeyDownListener:function(){this.documentKeydownListener||(this.documentKeydownListener=this.onKeydown,document.addEventListener(`keydown`,this.documentKeydownListener))},unbindDocumentKeyDownListener:function(){this.documentKeydownListener&&=(document.removeEventListener(`keydown`,this.documentKeydownListener),null)},bindOutsideClickListener:function(){var e=this;this.outsideClickListener||(this.outsideClickListener=function(t){e.isOutsideClicked(t)&&e.hide()},document.addEventListener(`click`,this.outsideClickListener,!0))},unbindOutsideClickListener:function(){this.outsideClickListener&&=(document.removeEventListener(`click`,this.outsideClickListener,!0),null)},isOutsideClicked:function(e){return this.container&&!this.container.contains(e.target)}},computed:{fullScreen:function(){return this.position===`full`},closeAriaLabel:function(){return this.$primevue.config.locale.aria?this.$primevue.config.locale.aria.close:void 0},dataP:function(){return L(We(We(We({"full-screen":this.position===`full`},this.position,this.position),`open`,this.containerVisible),`modal`,this.modal))}},directives:{focustrap:ae},components:{Button:z,Portal:R,TimesIcon:ce}},Je=[`data-p`],Ye=[`role`,`aria-modal`,`data-p`];function Xe(e,t,r,s,_,v){var y=i(`Button`),b=i(`Portal`),x=a(`focustrap`);return o(),h(b,null,{default:f(function(){return[_.containerVisible?(o(),C(`div`,u({key:0,ref:v.maskRef,onMousedown:t[0]||=function(){return v.onMaskClick&&v.onMaskClick.apply(v,arguments)},class:e.cx(`mask`),style:e.sx(`mask`,!0,{position:e.position,modal:e.modal}),"data-p":v.dataP},e.ptm(`mask`)),[m(O,u({name:`p-drawer`,onEnter:v.onEnter,onAfterEnter:v.onAfterEnter,onBeforeLeave:v.onBeforeLeave,onLeave:v.onLeave,onAfterLeave:v.onAfterLeave,appear:``},e.ptm(`transition`)),{default:f(function(){return[e.visible?p((o(),C(`div`,u({key:0,ref:v.containerRef,class:e.cx(`root`),style:e.sx(`root`),role:e.modal?`dialog`:`complementary`,"aria-modal":e.modal?!0:void 0,"data-p":v.dataP},e.ptmi(`root`)),[e.$slots.container?n(e.$slots,`container`,{key:0,closeCallback:v.hide}):(o(),C(S,{key:1},[w(`div`,u({ref:v.headerContainerRef,class:e.cx(`header`)},e.ptm(`header`)),[n(e.$slots,`header`,{class:l(e.cx(`title`))},function(){return[e.header?(o(),C(`div`,u({key:0,class:e.cx(`title`)},e.ptm(`title`)),d(e.header),17)):g(``,!0)]}),e.showCloseIcon?n(e.$slots,`closebutton`,{key:0,closeCallback:v.hide},function(){return[m(y,u({ref:v.closeButtonRef,type:`button`,class:e.cx(`pcCloseButton`),"aria-label":v.closeAriaLabel,unstyled:e.unstyled,onClick:v.hide},e.closeButtonProps,{pt:e.ptm(`pcCloseButton`),"data-pc-group-section":`iconcontainer`}),{icon:f(function(t){return[n(e.$slots,`closeicon`,{},function(){return[(o(),h(c(e.closeIcon?`span`:`TimesIcon`),u({class:[e.closeIcon,t.class]},e.ptm(`pcCloseButton`).icon),null,16,[`class`]))]})]}),_:3},16,[`class`,`aria-label`,`unstyled`,`onClick`,`pt`])]}):g(``,!0)],16),w(`div`,u({ref:v.contentRef,class:e.cx(`content`)},e.ptm(`content`)),[n(e.$slots,`default`)],16),e.$slots.footer?(o(),C(`div`,u({key:0,ref:v.footerContainerRef,class:e.cx(`footer`)},e.ptm(`footer`)),[n(e.$slots,`footer`)],16)):g(``,!0)],64))],16,Ye)),[[x]]):g(``,!0)]}),_:3},16,[`onEnter`,`onAfterEnter`,`onBeforeLeave`,`onLeave`,`onAfterLeave`])],16,Je)):g(``,!0)]}),_:3})}qe.render=Xe;var Ze=V.extend({name:`menu`,style:`
    .p-menu {
        background: dt('menu.background');
        color: dt('menu.color');
        border: 1px solid dt('menu.border.color');
        border-radius: dt('menu.border.radius');
        min-width: 12.5rem;
    }

    .p-menu-list {
        margin: 0;
        padding: dt('menu.list.padding');
        outline: 0 none;
        list-style: none;
        display: flex;
        flex-direction: column;
        gap: dt('menu.list.gap');
    }

    .p-menu-item-content {
        transition:
            background dt('menu.transition.duration'),
            color dt('menu.transition.duration');
        border-radius: dt('menu.item.border.radius');
        color: dt('menu.item.color');
        overflow: hidden;
    }

    .p-menu-item-link {
        cursor: pointer;
        display: flex;
        align-items: center;
        text-decoration: none;
        overflow: hidden;
        position: relative;
        color: inherit;
        padding: dt('menu.item.padding');
        gap: dt('menu.item.gap');
        user-select: none;
        outline: 0 none;
    }

    .p-menu-item-label {
        line-height: 1;
    }

    .p-menu-item-icon {
        color: dt('menu.item.icon.color');
    }

    .p-menu-item.p-focus .p-menu-item-content {
        color: dt('menu.item.focus.color');
        background: dt('menu.item.focus.background');
    }

    .p-menu-item.p-focus .p-menu-item-icon {
        color: dt('menu.item.icon.focus.color');
    }

    .p-menu-item:not(.p-disabled) .p-menu-item-content:hover {
        color: dt('menu.item.focus.color');
        background: dt('menu.item.focus.background');
    }

    .p-menu-item:not(.p-disabled) .p-menu-item-content:hover .p-menu-item-icon {
        color: dt('menu.item.icon.focus.color');
    }

    .p-menu-overlay {
        box-shadow: dt('menu.shadow');
    }

    .p-menu-submenu-label {
        background: dt('menu.submenu.label.background');
        padding: dt('menu.submenu.label.padding');
        color: dt('menu.submenu.label.color');
        font-weight: dt('menu.submenu.label.font.weight');
    }

    .p-menu-separator {
        border-block-start: 1px solid dt('menu.separator.border.color');
    }
`,classes:{root:function(e){return[`p-menu p-component`,{"p-menu-overlay":e.props.popup}]},start:`p-menu-start`,list:`p-menu-list`,submenuLabel:`p-menu-submenu-label`,separator:`p-menu-separator`,end:`p-menu-end`,item:function(e){var t=e.instance;return[`p-menu-item`,{"p-focus":t.id===t.focusedOptionId,"p-disabled":t.disabled()}]},itemContent:`p-menu-item-content`,itemLink:`p-menu-item-link`,itemIcon:`p-menu-item-icon`,itemLabel:`p-menu-item-label`}}),Qe={name:`BaseMenu`,extends:F,props:{popup:{type:Boolean,default:!1},model:{type:Array,default:null},appendTo:{type:[String,Object],default:`body`},autoZIndex:{type:Boolean,default:!0},baseZIndex:{type:Number,default:0},tabindex:{type:Number,default:0},ariaLabel:{type:String,default:null},ariaLabelledby:{type:String,default:null}},style:Ze,provide:function(){return{$pcMenu:this,$parentInstance:this}}},$e={name:`Menuitem`,hostName:`Menu`,extends:F,inheritAttrs:!1,emits:[`item-click`,`item-mousemove`],props:{item:null,templates:null,id:null,focusedOptionId:null,index:null},methods:{getItemProp:function(e,t){return e&&e.item?D(e.item[t]):void 0},getPTOptions:function(e){return this.ptm(e,{context:{item:this.item,index:this.index,focused:this.isItemFocused(),disabled:this.disabled()}})},isItemFocused:function(){return this.focusedOptionId===this.id},onItemClick:function(e){var t=this.getItemProp(this.item,`command`);t&&t({originalEvent:e,item:this.item.item}),this.$emit(`item-click`,{originalEvent:e,item:this.item,id:this.id})},onItemMouseMove:function(e){this.$emit(`item-mousemove`,{originalEvent:e,item:this.item,id:this.id})},visible:function(){return typeof this.item.visible==`function`?this.item.visible():this.item.visible!==!1},disabled:function(){return typeof this.item.disabled==`function`?this.item.disabled():this.item.disabled},label:function(){return typeof this.item.label==`function`?this.item.label():this.item.label},getMenuItemProps:function(e){return{action:u({class:this.cx(`itemLink`),tabindex:`-1`},this.getPTOptions(`itemLink`)),icon:u({class:[this.cx(`itemIcon`),e.icon]},this.getPTOptions(`itemIcon`)),label:u({class:this.cx(`itemLabel`)},this.getPTOptions(`itemLabel`))}}},computed:{dataP:function(){return L({focus:this.isItemFocused(),disabled:this.disabled()})}},directives:{ripple:N}},et=[`id`,`aria-label`,`aria-disabled`,`data-p-focused`,`data-p-disabled`,`data-p`],tt=[`data-p`],nt=[`href`,`target`],rt=[`data-p`],it=[`data-p`];function at(e,t,n,r,i,s){var f=a(`ripple`);return s.visible()?(o(),C(`li`,u({key:0,id:n.id,class:[e.cx(`item`),n.item.class],role:`menuitem`,style:n.item.style,"aria-label":s.label(),"aria-disabled":s.disabled(),"data-p-focused":s.isItemFocused(),"data-p-disabled":s.disabled()||!1,"data-p":s.dataP},s.getPTOptions(`item`)),[w(`div`,u({class:e.cx(`itemContent`),onClick:t[0]||=function(e){return s.onItemClick(e)},onMousemove:t[1]||=function(e){return s.onItemMouseMove(e)},"data-p":s.dataP},s.getPTOptions(`itemContent`)),[n.templates.item?n.templates.item?(o(),h(c(n.templates.item),{key:1,item:n.item,label:s.label(),props:s.getMenuItemProps(n.item)},null,8,[`item`,`label`,`props`])):g(``,!0):p((o(),C(`a`,u({key:0,href:n.item.url,class:e.cx(`itemLink`),target:n.item.target,tabindex:`-1`},s.getPTOptions(`itemLink`)),[n.templates.itemicon?(o(),h(c(n.templates.itemicon),{key:0,item:n.item,class:l(e.cx(`itemIcon`))},null,8,[`item`,`class`])):n.item.icon?(o(),C(`span`,u({key:1,class:[e.cx(`itemIcon`),n.item.icon],"data-p":s.dataP},s.getPTOptions(`itemIcon`)),null,16,rt)):g(``,!0),w(`span`,u({class:e.cx(`itemLabel`),"data-p":s.dataP},s.getPTOptions(`itemLabel`)),d(s.label()),17,it)],16,nt)),[[f]])],16,tt)],16,et)):g(``,!0)}$e.render=at;function X(e){return Z(e)||ct(e)||st(e)||ot()}function ot(){throw TypeError(`Invalid attempt to spread non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`)}function st(e,t){if(e){if(typeof e==`string`)return lt(e,t);var n={}.toString.call(e).slice(8,-1);return n===`Object`&&e.constructor&&(n=e.constructor.name),n===`Map`||n===`Set`?Array.from(e):n===`Arguments`||/^(?:Ui|I)nt(?:8|16|32)(?:Clamped)?Array$/.test(n)?lt(e,t):void 0}}function ct(e){if(typeof Symbol<`u`&&e[Symbol.iterator]!=null||e[`@@iterator`]!=null)return Array.from(e)}function Z(e){if(Array.isArray(e))return lt(e)}function lt(e,t){(t==null||t>e.length)&&(t=e.length);for(var n=0,r=Array(t);n<t;n++)r[n]=e[n];return r}var ut={name:`Menu`,extends:Qe,inheritAttrs:!1,emits:[`show`,`hide`,`focus`,`blur`],data:function(){return{overlayVisible:!1,focused:!1,focusedOptionIndex:-1,selectedOptionIndex:-1}},target:null,outsideClickListener:null,scrollHandler:null,resizeListener:null,container:null,list:null,mounted:function(){this.popup||(this.bindResizeListener(),this.bindOutsideClickListener())},beforeUnmount:function(){this.unbindResizeListener(),this.unbindOutsideClickListener(),this.scrollHandler&&=(this.scrollHandler.destroy(),null),this.target=null,this.container&&this.autoZIndex&&A.clear(this.container),this.container=null},methods:{itemClick:function(e){var t=e.item;this.disabled(t)||(t.command&&t.command(e),this.overlayVisible&&this.hide(),!this.popup&&this.focusedOptionIndex!==e.id&&(this.focusedOptionIndex=e.id))},itemMouseMove:function(e){this.focused&&(this.focusedOptionIndex=e.id)},onListFocus:function(e){this.focused=!0,!this.popup&&this.changeFocusedOptionIndex(0),this.$emit(`focus`,e)},onListBlur:function(e){this.focused=!1,this.focusedOptionIndex=-1,this.$emit(`blur`,e)},onListKeyDown:function(e){switch(e.code){case`ArrowDown`:this.onArrowDownKey(e);break;case`ArrowUp`:this.onArrowUpKey(e);break;case`Home`:this.onHomeKey(e);break;case`End`:this.onEndKey(e);break;case`Enter`:case`NumpadEnter`:this.onEnterKey(e);break;case`Space`:this.onSpaceKey(e);break;case`Escape`:this.popup&&(de(this.target),this.hide());case`Tab`:this.overlayVisible&&this.hide()}},onArrowDownKey:function(e){var t=this.findNextOptionIndex(this.focusedOptionIndex);this.changeFocusedOptionIndex(t),e.preventDefault()},onArrowUpKey:function(e){if(e.altKey&&this.popup)de(this.target),this.hide(),e.preventDefault();else{var t=this.findPrevOptionIndex(this.focusedOptionIndex);this.changeFocusedOptionIndex(t),e.preventDefault()}},onHomeKey:function(e){this.changeFocusedOptionIndex(0),e.preventDefault()},onEndKey:function(e){this.changeFocusedOptionIndex(re(this.container,`li[data-pc-section="item"][data-p-disabled="false"]`).length-1),e.preventDefault()},onEnterKey:function(e){var t=P(this.list,`li[id="${`${this.focusedOptionIndex}`}"]`),n=t&&P(t,`a[data-pc-section="itemlink"]`);this.popup&&de(this.target),n?n.click():t&&t.click(),e.preventDefault()},onSpaceKey:function(e){this.onEnterKey(e)},findNextOptionIndex:function(e){var t=X(re(this.container,`li[data-pc-section="item"][data-p-disabled="false"]`)).findIndex(function(t){return t.id===e});return t>-1?t+1:0},findPrevOptionIndex:function(e){var t=X(re(this.container,`li[data-pc-section="item"][data-p-disabled="false"]`)).findIndex(function(t){return t.id===e});return t>-1?t-1:0},changeFocusedOptionIndex:function(e){var t=re(this.container,`li[data-pc-section="item"][data-p-disabled="false"]`),n=e>=t.length?t.length-1:e<0?0:e;n>-1&&(this.focusedOptionIndex=t[n].getAttribute(`id`))},toggle:function(e,t){this.overlayVisible?this.hide():this.show(e,t)},show:function(e,t){this.overlayVisible=!0,this.target=t??e.currentTarget},hide:function(){this.overlayVisible=!1,this.target=null},onEnter:function(e){ne(e,{position:`absolute`,top:`0`}),this.alignOverlay(),this.bindOutsideClickListener(),this.bindResizeListener(),this.bindScrollListener(),this.autoZIndex&&A.set(`menu`,e,this.baseZIndex||this.$primevue.config.zIndex.menu),this.popup&&de(this.list),this.$emit(`show`)},onLeave:function(){this.unbindOutsideClickListener(),this.unbindResizeListener(),this.unbindScrollListener(),this.$emit(`hide`)},onAfterLeave:function(e){this.autoZIndex&&A.clear(e)},alignOverlay:function(){I(this.container,this.target),fe(this.target)>fe(this.container)&&(this.container.style.minWidth=fe(this.target)+`px`)},bindOutsideClickListener:function(){var e=this;this.outsideClickListener||(this.outsideClickListener=function(t){var n=e.container&&!e.container.contains(t.target),r=!e.target||e.target!==t.target&&!e.target.contains(t.target);e.overlayVisible&&n&&r?e.hide():!e.popup&&n&&r&&(e.focusedOptionIndex=-1)},document.addEventListener(`click`,this.outsideClickListener,!0))},unbindOutsideClickListener:function(){this.outsideClickListener&&=(document.removeEventListener(`click`,this.outsideClickListener,!0),null)},bindScrollListener:function(){var e=this;this.scrollHandler||=new Re(this.target,function(){e.overlayVisible&&e.hide()}),this.scrollHandler.bindScrollListener()},unbindScrollListener:function(){this.scrollHandler&&this.scrollHandler.unbindScrollListener()},bindResizeListener:function(){var e=this;this.resizeListener||(this.resizeListener=function(){e.overlayVisible&&!E()&&e.hide()},window.addEventListener(`resize`,this.resizeListener))},unbindResizeListener:function(){this.resizeListener&&=(window.removeEventListener(`resize`,this.resizeListener),null)},visible:function(e){return typeof e.visible==`function`?e.visible():e.visible!==!1},disabled:function(e){return typeof e.disabled==`function`?e.disabled():e.disabled},label:function(e){return typeof e.label==`function`?e.label():e.label},onOverlayClick:function(e){ze.emit(`overlay-click`,{originalEvent:e,target:this.target})},containerRef:function(e){this.container=e},listRef:function(e){this.list=e}},computed:{focusedOptionId:function(){return this.focusedOptionIndex===-1?null:this.focusedOptionIndex},dataP:function(){return L({popup:this.popup})}},components:{PVMenuitem:$e,Portal:R}},dt=[`id`,`data-p`],ft=[`id`,`tabindex`,`aria-activedescendant`,`aria-label`,`aria-labelledby`],pt=[`id`];function mt(e,t,r,a,c,l){var p=i(`PVMenuitem`),v=i(`Portal`);return o(),h(v,{appendTo:e.appendTo,disabled:!e.popup},{default:f(function(){return[m(O,u({name:`p-anchored-overlay`,onEnter:l.onEnter,onLeave:l.onLeave,onAfterLeave:l.onAfterLeave},e.ptm(`transition`)),{default:f(function(){return[!e.popup||c.overlayVisible?(o(),C(`div`,u({key:0,ref:l.containerRef,id:e.$id,class:e.cx(`root`),onClick:t[3]||=function(){return l.onOverlayClick&&l.onOverlayClick.apply(l,arguments)},"data-p":l.dataP},e.ptmi(`root`)),[e.$slots.start?(o(),C(`div`,u({key:0,class:e.cx(`start`)},e.ptm(`start`)),[n(e.$slots,`start`)],16)):g(``,!0),w(`ul`,u({ref:l.listRef,id:e.$id+`_list`,class:e.cx(`list`),role:`menu`,tabindex:e.tabindex,"aria-activedescendant":c.focused?l.focusedOptionId:void 0,"aria-label":e.ariaLabel,"aria-labelledby":e.ariaLabelledby,onFocus:t[0]||=function(){return l.onListFocus&&l.onListFocus.apply(l,arguments)},onBlur:t[1]||=function(){return l.onListBlur&&l.onListBlur.apply(l,arguments)},onKeydown:t[2]||=function(){return l.onListKeyDown&&l.onListKeyDown.apply(l,arguments)}},e.ptm(`list`)),[(o(!0),C(S,null,s(e.model,function(t,r){return o(),C(S,{key:l.label(t)+r.toString()},[t.items&&l.visible(t)&&!t.separator?(o(),C(S,{key:0},[t.items?(o(),C(`li`,u({key:0,id:e.$id+`_`+r,class:[e.cx(`submenuLabel`),t.class],role:`none`},{ref_for:!0},e.ptm(`submenuLabel`)),[n(e.$slots,e.$slots.submenulabel?`submenulabel`:`submenuheader`,{item:t},function(){return[_(d(l.label(t)),1)]})],16,pt)):g(``,!0),(o(!0),C(S,null,s(t.items,function(n,i){return o(),C(S,{key:n.label+r+`_`+i},[l.visible(n)&&!n.separator?(o(),h(p,{key:0,id:e.$id+`_`+r+`_`+i,item:n,templates:e.$slots,focusedOptionId:l.focusedOptionId,unstyled:e.unstyled,onItemClick:l.itemClick,onItemMousemove:l.itemMouseMove,pt:e.pt},null,8,[`id`,`item`,`templates`,`focusedOptionId`,`unstyled`,`onItemClick`,`onItemMousemove`,`pt`])):l.visible(n)&&n.separator?(o(),C(`li`,u({key:`separator`+r+i,class:[e.cx(`separator`),t.class],style:n.style,role:`separator`},{ref_for:!0},e.ptm(`separator`)),null,16)):g(``,!0)],64)}),128))],64)):l.visible(t)&&t.separator?(o(),C(`li`,u({key:`separator`+r.toString(),class:[e.cx(`separator`),t.class],style:t.style,role:`separator`},{ref_for:!0},e.ptm(`separator`)),null,16)):(o(),h(p,{key:l.label(t)+r.toString(),id:e.$id+`_`+r,item:t,index:r,templates:e.$slots,focusedOptionId:l.focusedOptionId,unstyled:e.unstyled,onItemClick:l.itemClick,onItemMousemove:l.itemMouseMove,pt:e.pt},null,8,[`id`,`item`,`index`,`templates`,`focusedOptionId`,`unstyled`,`onItemClick`,`onItemMousemove`,`pt`]))],64)}),128))],16,ft),e.$slots.end?(o(),C(`div`,u({key:1,class:e.cx(`end`)},e.ptm(`end`)),[n(e.$slots,`end`)],16)):g(``,!0)],16,dt)):g(``,!0)]}),_:3},16,[`onEnter`,`onLeave`,`onAfterLeave`])]}),_:3},8,[`appendTo`,`disabled`])}ut.render=mt;var ht=V.extend({name:`togglebutton`,style:`
    .p-togglebutton {
        display: inline-flex;
        cursor: pointer;
        user-select: none;
        overflow: hidden;
        position: relative;
        color: dt('togglebutton.color');
        background: dt('togglebutton.background');
        border: 1px solid dt('togglebutton.border.color');
        padding: dt('togglebutton.padding');
        font-size: 1rem;
        font-family: inherit;
        font-feature-settings: inherit;
        transition:
            background dt('togglebutton.transition.duration'),
            color dt('togglebutton.transition.duration'),
            border-color dt('togglebutton.transition.duration'),
            outline-color dt('togglebutton.transition.duration'),
            box-shadow dt('togglebutton.transition.duration');
        border-radius: dt('togglebutton.border.radius');
        outline-color: transparent;
        font-weight: dt('togglebutton.font.weight');
    }

    .p-togglebutton-content {
        display: inline-flex;
        flex: 1 1 auto;
        align-items: center;
        justify-content: center;
        gap: dt('togglebutton.gap');
        padding: dt('togglebutton.content.padding');
        background: transparent;
        border-radius: dt('togglebutton.content.border.radius');
        transition:
            background dt('togglebutton.transition.duration'),
            color dt('togglebutton.transition.duration'),
            border-color dt('togglebutton.transition.duration'),
            outline-color dt('togglebutton.transition.duration'),
            box-shadow dt('togglebutton.transition.duration');
    }

    .p-togglebutton:not(:disabled):not(.p-togglebutton-checked):hover {
        background: dt('togglebutton.hover.background');
        color: dt('togglebutton.hover.color');
    }

    .p-togglebutton.p-togglebutton-checked {
        background: dt('togglebutton.checked.background');
        border-color: dt('togglebutton.checked.border.color');
        color: dt('togglebutton.checked.color');
    }

    .p-togglebutton-checked .p-togglebutton-content {
        background: dt('togglebutton.content.checked.background');
        box-shadow: dt('togglebutton.content.checked.shadow');
    }

    .p-togglebutton:focus-visible {
        box-shadow: dt('togglebutton.focus.ring.shadow');
        outline: dt('togglebutton.focus.ring.width') dt('togglebutton.focus.ring.style') dt('togglebutton.focus.ring.color');
        outline-offset: dt('togglebutton.focus.ring.offset');
    }

    .p-togglebutton.p-invalid {
        border-color: dt('togglebutton.invalid.border.color');
    }

    .p-togglebutton:disabled {
        opacity: 1;
        cursor: default;
        background: dt('togglebutton.disabled.background');
        border-color: dt('togglebutton.disabled.border.color');
        color: dt('togglebutton.disabled.color');
    }

    .p-togglebutton-label,
    .p-togglebutton-icon {
        position: relative;
        transition: none;
    }

    .p-togglebutton-icon {
        color: dt('togglebutton.icon.color');
    }

    .p-togglebutton:not(:disabled):not(.p-togglebutton-checked):hover .p-togglebutton-icon {
        color: dt('togglebutton.icon.hover.color');
    }

    .p-togglebutton.p-togglebutton-checked .p-togglebutton-icon {
        color: dt('togglebutton.icon.checked.color');
    }

    .p-togglebutton:disabled .p-togglebutton-icon {
        color: dt('togglebutton.icon.disabled.color');
    }

    .p-togglebutton-sm {
        padding: dt('togglebutton.sm.padding');
        font-size: dt('togglebutton.sm.font.size');
    }

    .p-togglebutton-sm .p-togglebutton-content {
        padding: dt('togglebutton.content.sm.padding');
    }

    .p-togglebutton-lg {
        padding: dt('togglebutton.lg.padding');
        font-size: dt('togglebutton.lg.font.size');
    }

    .p-togglebutton-lg .p-togglebutton-content {
        padding: dt('togglebutton.content.lg.padding');
    }

    .p-togglebutton-fluid {
        width: 100%;
    }
`,classes:{root:function(e){var t=e.instance,n=e.props;return[`p-togglebutton p-component`,{"p-togglebutton-checked":t.active,"p-invalid":t.$invalid,"p-togglebutton-fluid":n.fluid,"p-togglebutton-sm p-inputfield-sm":n.size===`small`,"p-togglebutton-lg p-inputfield-lg":n.size===`large`}]},content:`p-togglebutton-content`,icon:`p-togglebutton-icon`,label:`p-togglebutton-label`}}),gt={name:`BaseToggleButton`,extends:ge,props:{onIcon:String,offIcon:String,onLabel:{type:String,default:`Yes`},offLabel:{type:String,default:`No`},readonly:{type:Boolean,default:!1},tabindex:{type:Number,default:null},ariaLabelledby:{type:String,default:null},ariaLabel:{type:String,default:null},size:{type:String,default:null},fluid:{type:Boolean,default:null}},style:ht,provide:function(){return{$pcToggleButton:this,$parentInstance:this}}};function Q(e){"@babel/helpers - typeof";return Q=typeof Symbol==`function`&&typeof Symbol.iterator==`symbol`?function(e){return typeof e}:function(e){return e&&typeof Symbol==`function`&&e.constructor===Symbol&&e!==Symbol.prototype?`symbol`:typeof e},Q(e)}function _t(e,t,n){return(t=vt(t))in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function vt(e){var t=yt(e,`string`);return Q(t)==`symbol`?t:t+``}function yt(e,t){if(Q(e)!=`object`||!e)return e;var n=e[Symbol.toPrimitive];if(n!==void 0){var r=n.call(e,t);if(Q(r)!=`object`)return r;throw TypeError(`@@toPrimitive must return a primitive value.`)}return(t===`string`?String:Number)(e)}var bt={name:`ToggleButton`,extends:gt,inheritAttrs:!1,emits:[`change`],methods:{getPTOptions:function(e){return(e===`root`?this.ptmi:this.ptm)(e,{context:{active:this.active,disabled:this.disabled}})},onChange:function(e){!this.disabled&&!this.readonly&&(this.writeValue(!this.d_value,e),this.$emit(`change`,e))},onBlur:function(e){var t,n;(t=(n=this.formField).onBlur)==null||t.call(n,e)}},computed:{active:function(){return this.d_value===!0},hasLabel:function(){return k(this.onLabel)&&k(this.offLabel)},label:function(){return this.hasLabel?this.d_value?this.onLabel:this.offLabel:`\xA0`},dataP:function(){return L(_t({checked:this.active,invalid:this.$invalid},this.size,this.size))}},directives:{ripple:N}},xt=[`tabindex`,`disabled`,`aria-pressed`,`aria-label`,`aria-labelledby`,`data-p-checked`,`data-p-disabled`,`data-p`],St=[`data-p`];function Ct(e,t,r,i,s,c){var f=a(`ripple`);return p((o(),C(`button`,u({type:`button`,class:e.cx(`root`),tabindex:e.tabindex,disabled:e.disabled,"aria-pressed":e.d_value,onClick:t[0]||=function(){return c.onChange&&c.onChange.apply(c,arguments)},onBlur:t[1]||=function(){return c.onBlur&&c.onBlur.apply(c,arguments)}},c.getPTOptions(`root`),{"aria-label":e.ariaLabel,"aria-labelledby":e.ariaLabelledby,"data-p-checked":c.active,"data-p-disabled":e.disabled,"data-p":c.dataP}),[w(`span`,u({class:e.cx(`content`)},c.getPTOptions(`content`),{"data-p":c.dataP}),[n(e.$slots,`default`,{},function(){return[n(e.$slots,`icon`,{value:e.d_value,class:l(e.cx(`icon`))},function(){return[e.onIcon||e.offIcon?(o(),C(`span`,u({key:0,class:[e.cx(`icon`),e.d_value?e.onIcon:e.offIcon]},c.getPTOptions(`icon`)),null,16)):g(``,!0)]}),w(`span`,u({class:e.cx(`label`)},c.getPTOptions(`label`)),d(c.label),17)]})],16,St)],16,xt)),[[f]])}bt.render=Ct;var wt=V.extend({name:`selectbutton`,style:`
    .p-selectbutton {
        display: inline-flex;
        user-select: none;
        vertical-align: bottom;
        outline-color: transparent;
        border-radius: dt('selectbutton.border.radius');
    }

    .p-selectbutton .p-togglebutton {
        border-radius: 0;
        border-width: 1px 1px 1px 0;
    }

    .p-selectbutton .p-togglebutton:focus-visible {
        position: relative;
        z-index: 1;
    }

    .p-selectbutton .p-togglebutton:first-child {
        border-inline-start-width: 1px;
        border-start-start-radius: dt('selectbutton.border.radius');
        border-end-start-radius: dt('selectbutton.border.radius');
    }

    .p-selectbutton .p-togglebutton:last-child {
        border-start-end-radius: dt('selectbutton.border.radius');
        border-end-end-radius: dt('selectbutton.border.radius');
    }

    .p-selectbutton.p-invalid {
        outline: 1px solid dt('selectbutton.invalid.border.color');
        outline-offset: 0;
    }

    .p-selectbutton-fluid {
        width: 100%;
    }
    
    .p-selectbutton-fluid .p-togglebutton {
        flex: 1 1 0;
    }
`,classes:{root:function(e){var t=e.props;return[`p-selectbutton p-component`,{"p-invalid":e.instance.$invalid,"p-selectbutton-fluid":t.fluid}]}}}),Tt={name:`BaseSelectButton`,extends:ge,props:{options:Array,optionLabel:null,optionValue:null,optionDisabled:null,multiple:Boolean,allowEmpty:{type:Boolean,default:!0},dataKey:null,ariaLabelledby:{type:String,default:null},size:{type:String,default:null},fluid:{type:Boolean,default:null}},style:wt,provide:function(){return{$pcSelectButton:this,$parentInstance:this}}};function Et(e,t){var n=typeof Symbol<`u`&&e[Symbol.iterator]||e[`@@iterator`];if(!n){if(Array.isArray(e)||(n=kt(e))||t){n&&(e=n);var r=0,i=function(){};return{s:i,n:function(){return r>=e.length?{done:!0}:{done:!1,value:e[r++]}},e:function(e){throw e},f:i}}throw TypeError(`Invalid attempt to iterate non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`)}var a,o=!0,s=!1;return{s:function(){n=n.call(e)},n:function(){var e=n.next();return o=e.done,e},e:function(e){s=!0,a=e},f:function(){try{o||n.return==null||n.return()}finally{if(s)throw a}}}}function Dt(e){return jt(e)||At(e)||kt(e)||Ot()}function Ot(){throw TypeError(`Invalid attempt to spread non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`)}function kt(e,t){if(e){if(typeof e==`string`)return Mt(e,t);var n={}.toString.call(e).slice(8,-1);return n===`Object`&&e.constructor&&(n=e.constructor.name),n===`Map`||n===`Set`?Array.from(e):n===`Arguments`||/^(?:Ui|I)nt(?:8|16|32)(?:Clamped)?Array$/.test(n)?Mt(e,t):void 0}}function At(e){if(typeof Symbol<`u`&&e[Symbol.iterator]!=null||e[`@@iterator`]!=null)return Array.from(e)}function jt(e){if(Array.isArray(e))return Mt(e)}function Mt(e,t){(t==null||t>e.length)&&(t=e.length);for(var n=0,r=Array(t);n<t;n++)r[n]=e[n];return r}var Nt={name:`SelectButton`,extends:Tt,inheritAttrs:!1,emits:[`change`],methods:{getOptionLabel:function(e){return this.optionLabel?B(e,this.optionLabel):e},getOptionValue:function(e){return this.optionValue?B(e,this.optionValue):e},getOptionRenderKey:function(e){return this.dataKey?B(e,this.dataKey):this.getOptionLabel(e)},isOptionDisabled:function(e){return this.optionDisabled?B(e,this.optionDisabled):!1},isOptionReadonly:function(e){if(this.allowEmpty)return!1;var t=this.isSelected(e);return this.multiple?t&&this.d_value.length===1:t},onOptionSelect:function(e,t,n){var r=this;if(!(this.disabled||this.isOptionDisabled(t)||this.isOptionReadonly(t))){var i=this.isSelected(t),a=this.getOptionValue(t),o;if(this.multiple){if(i){if(o=this.d_value.filter(function(e){return!me(e,a,r.equalityKey)}),!this.allowEmpty&&o.length===0)return}else o=this.d_value?[].concat(Dt(this.d_value),[a]):[a]}else{if(i&&!this.allowEmpty)return;o=i?null:a}this.writeValue(o,e),this.$emit(`change`,{originalEvent:e,value:o})}},isSelected:function(e){var t=!1,n=this.getOptionValue(e);if(this.multiple){if(this.d_value){var r=Et(this.d_value),i;try{for(r.s();!(i=r.n()).done;){var a=i.value;if(me(a,n,this.equalityKey)){t=!0;break}}}catch(e){r.e(e)}finally{r.f()}}}else t=me(this.d_value,n,this.equalityKey);return t}},computed:{equalityKey:function(){return this.optionValue?null:this.dataKey},dataP:function(){return L({invalid:this.$invalid})}},directives:{ripple:N},components:{ToggleButton:bt}},Pt=[`aria-labelledby`,`data-p`];function Ft(e,t,r,a,c,l){var p=i(`ToggleButton`);return o(),C(`div`,u({class:e.cx(`root`),role:`group`,"aria-labelledby":e.ariaLabelledby},e.ptmi(`root`),{"data-p":l.dataP}),[(o(!0),C(S,null,s(e.options,function(t,r){return o(),h(p,{key:l.getOptionRenderKey(t),modelValue:l.isSelected(t),onLabel:l.getOptionLabel(t),offLabel:l.getOptionLabel(t),disabled:e.disabled||l.isOptionDisabled(t),unstyled:e.unstyled,size:e.size,readonly:l.isOptionReadonly(t),onChange:function(e){return l.onOptionSelect(e,t,r)},pt:e.ptm(`pcToggleButton`)},v({_:2},[e.$slots.option?{name:`default`,fn:f(function(){return[n(e.$slots,`option`,{option:t,index:r},function(){return[w(`span`,u({ref_for:!0},e.ptm(`pcToggleButton`).label),d(l.getOptionLabel(t)),17)]})]}),key:`0`}:void 0]),1032,[`modelValue`,`onLabel`,`offLabel`,`disabled`,`unstyled`,`size`,`readonly`,`onChange`,`pt`])}),128))],16,Pt)}Nt.render=Ft;var It={class:`findings`},Lt={class:`findings-head`},Rt={class:`stale`},zt={key:1,class:`hint`},Bt={key:0,class:`empty`},Vt={key:1},Ht={class:`finding`},Ut={class:`kind`},Wt={class:`client`},Gt={class:`login`},Kt={class:`client`},qt={key:0},Jt={key:1},Yt={key:1,class:`client`},Xt=le(T({__name:`FindingsPanel`,props:{search:{}},emits:[`clear-search`,`changed`,`open`],setup(t,{expose:n,emit:r}){let i=t,a=r,s=e([]),c=e(`active`),l=b(()=>i.search),u=e(null),p=e(0),v=De,T=G;function ee(e){return e.acknowledged||e.suppressed===!0}let{loading:te,error:E,load:D}=ve(async()=>{s.value=(await x.anomalies(!0)).anomalies??[]},{failText:`Не удалось загрузить находки.`}),O=b(()=>s.value.filter(e=>!ee(e))),k=b(()=>s.value.filter(ee)),ne=b(()=>k.value.filter(e=>e.reviewDue).length),re=b(()=>{let e=l.value.trim().toLowerCase(),t=c.value===`active`?O.value:k.value;return e?t.filter(t=>[t.clientName,t.inn,t.login,t.edoId].join(` `).toLowerCase().includes(e)):t}),A=Fe(()=>re.value,{resetOn:[l,c]}),j=b(()=>O.value.filter(e=>e.confidence===`high`)),ie=b(()=>l.value.trim()!==``);async function M(){u.value=null,await D(!0),a(`changed`)}async function ae(e){p.value=e.id;try{e.acknowledged&&await x.unacknowledge(e.edoId,e.fingerprint),e.suppressed&&await x.removeTopology({inn:e.inn,kpp:e.kpp,edoId:e.edoId}),await D(!0),a(`changed`)}catch(e){E.value=e instanceof Error?e.message:`Не удалось вернуть находку.`}finally{p.value=0}}function N({data:e}){let t=Ae(e.inn,e.kpp,e.edoId);t&&a(`open`,t)}return n({reload:()=>D(!0)}),(e,t)=>(o(),C(`section`,It,[w(`header`,Lt,[m(y(z),{label:`Активные · ${O.value.length}`,size:`small`,outlined:c.value!==`active`,onClick:t[0]||=e=>c.value=`active`},null,8,[`label`,`outlined`]),m(y(z),{label:`Скрытые · ${k.value.length}`,size:`small`,outlined:c.value!==`hidden`,onClick:t[1]||=e=>c.value=`hidden`},null,8,[`label`,`outlined`])]),y(E)?(o(),h(y(H),{key:0,severity:`error`,closable:!1},{default:f(()=>[_(d(y(E)),1)]),_:1})):(o(),C(S,{key:1},[!y(te)&&ne.value>0&&c.value===`active`?(o(),h(y(H),{key:0,severity:`warn`,closable:!1},{default:f(()=>[w(`span`,Rt,[_(` Скрытых находок пора пересмотреть: `+d(ne.value)+`. Пометки «это законно» живут полгода. `,1),m(y(z),{label:`Открыть скрытые`,size:`small`,text:``,onClick:t[2]||=e=>c.value=`hidden`})])]),_:1})):g(``,!0),c.value===`hidden`?(o(),C(`p`,zt,` Здесь находки, помеченные законными, и погашенные реестром ожидаемой топологии. «Вернуть» снимает пометку или убирает связь из реестра — находка снова станет активной. `)):g(``,!0),!y(te)&&c.value===`active`&&j.value.length===0?(o(),h(y(H),{key:2,severity:`success`,closable:!1},{default:f(()=>[...t[5]||=[_(` Срочных находок нет. `,-1)]]),_:1})):g(``,!0),m(Ne,{feed:y(A),loading:y(te),"data-key":`id`,class:`table`,onRowClick:N},{empty:f(()=>[ie.value?(o(),C(`div`,Bt,[t[6]||=w(`span`,null,`По запросу ничего не найдено.`,-1),m(y(z),{label:`Сбросить поиск`,size:`small`,text:``,onClick:t[3]||=e=>a(`clear-search`)})])):(o(),C(`span`,Vt,`Находок нет.`))]),default:f(()=>[m(y(J),{field:`confidence`,header:`Находка`,sortable:``,style:{width:`11rem`}},{body:f(({data:e})=>[w(`div`,Ht,[m(y(_e),{severity:e.confidence===`high`?`danger`:`secondary`,value:y(T)(e)},null,8,[`severity`,`value`]),w(`strong`,Ut,d(y(v)[e.kind]??e.kind),1)])]),_:1}),m(y(J),{field:`clientName`,header:`Клиент`,sortable:``},{body:f(({data:e})=>[m(Pe,{name:e.clientName,inn:e.inn},null,8,[`name`,`inn`])]),_:1}),m(y(J),{field:`edoId`,header:`Логин и идентификатор`,class:`wide-only`,style:{width:`19.5rem`}},{body:f(({data:e})=>[w(`div`,Wt,[w(`span`,Gt,d(e.login||`—`),1),w(`code`,null,d(e.edoId),1)])]),_:1}),m(y(J),{field:`details`,header:`Подробности`,style:{"min-width":`16rem`}},{body:f(({data:e})=>[w(`div`,Kt,[w(`span`,null,d(e.details),1),e.suppressed?(o(),C(`small`,qt,`Связь в реестре топологии: `+d(e.topologyPurpose),1)):g(``,!0),e.acknowledged?(o(),C(`small`,Jt,[_(` Законно: `+d(e.ackReason||`без причины`)+` · `+d(e.ackAuthor)+` `+d(y(xe)(e.ackedAt,``)),1),e.reviewAt?(o(),C(S,{key:0},[_(`, пересмотр `+d(y(xe)(e.reviewAt)),1)],64)):g(``,!0)])):g(``,!0)])]),_:1}),m(y(J),{header:`Действие`,style:{width:`12rem`}},{body:f(({data:e})=>[ee(e)?(o(),C(`div`,Yt,[e.reviewDue?(o(),h(y(_e),{key:0,value:`пора пересмотреть`,severity:`warn`})):g(``,!0),m(y(z),{label:`Вернуть`,size:`small`,severity:`secondary`,outlined:``,loading:p.value===e.id,onClick:se(t=>ae(e),[`stop`])},null,8,[`loading`,`onClick`])])):(o(),h(y(z),{key:0,label:`Это нормально`,size:`small`,severity:`secondary`,outlined:``,onClick:se(t=>u.value=e,[`stop`])},null,8,[`onClick`]))]),_:1})]),_:1},8,[`feed`,`loading`])],64)),m(U,{target:u.value,onClose:t[4]||=e=>u.value=null,onDone:M},null,8,[`target`])]))}}),[[`__scopeId`,`data-v-62dec8a8`]]),Zt=[`invoice`,`forecast`,`low`,`renewal`,`licenses`,`anomalies`,`tariff`,`industry`,`gap`,`notinbase`];function $(e,t,n,r,i){return{key:e,label:t,short:n,tone:r,items:i}}function Qt(e){return{invoice:$(`invoice`,`Выставить счёт`,`счёт`,`danger`,e.billing.overLimit.map(({clients:e,item:t})=>({clients:e,text:`сверх лимита ${t.billable}, счёт ${K(t.amount)}`,late:!0}))),forecast:$(`forecast`,e.forecast.period?`Выйдут за лимит в ${q(e.forecast.period)}`:`Выйдут за лимит`,`прогноз`,`danger`,e.forecast.items.map(({clients:e,item:t})=>({clients:e,text:W(t),late:t.exceeded}))),low:$(`low`,`Скоро кончится лимит`,`лимит`,`warn`,e.billing.lowRemainder.map(({clients:e,item:t})=>({clients:e,text:`израсходовано ${t.used} из ${t.limit??`—`}`,late:!1}))),renewal:$(`renewal`,`Продление 1С:ИТС`,`продление`,`warn`,e.renewals.items.map(({clients:e,item:t})=>({clients:e,text:`${Te(t.contract)} до ${Se(t.contract.end)}, ${we(t.daysLeft)}`,late:t.daysLeft<0}))),licenses:$(`licenses`,`Лицензии сервисов`,`лицензии`,`warn`,[...e.licenses.expiring.map(({clients:e,item:t})=>({clients:e,text:`${t.tariff.name} до ${Se(t.tariff.end)}, ${we(t.daysLeft)}`,late:t.daysLeft<0})),...e.licenses.low.map(({clients:e,item:t})=>({clients:e,text:`${Ce(t.option.type)}, ${t.tariffName}: ${je(t.option)}`,late:t.over}))]),anomalies:$(`anomalies`,`Находки ЭДО`,`находки`,`danger`,[...e.anomalies].sort((e,t)=>Number(t.item.confidence===`high`)-Number(e.item.confidence===`high`)).map(({clients:e,item:t})=>({clients:e,text:`${De[t.kind]??t.kind}: ${t.details}`,late:t.confidence===`high`}))),tariff:$(`tariff`,`Выгоднее другой тариф ЭПД`,`тариф`,`success`,e.advice.map(({clients:e,item:t})=>({clients:e,text:`${Oe(t)}: экономия ${K(t.savings)} в год`,late:!1}))),industry:$(`industry`,`Нужен ИТС Отраслевой`,`отраслевой`,`info`,e.industry.map(({clients:e,item:t})=>({clients:e,text:`нужен для «${t.programs.join(`», «`)}»`,late:!1}))),gap:$(`gap`,`ЭДО без биллинга`,`ЭДО без биллинга`,`warn`,e.gaps.edoWithoutBilling.map(({clients:t,item:n})=>({clients:t,text:`абонент ${n.code}: ${n.inTraffic?`трафик есть`:`направление ЭДО в 1С`}, в биллинге${e.gaps.period?` за ${q(e.gaps.period)}`:``} нет`,late:!1}))),notinbase:$(`notinbase`,`Нет в базе абонентов`,`нет в базе`,`info`,e.gaps.notInBase.map(({clients:e,item:t})=>({clients:e,text:`в биллинге, владелец ${t.ownerCodes.join(`, `)||`не указан`}; в базе абонентов 1С не найден`,late:!1})))}}var $t=[`invoice`,`forecast`,`anomalies`,`renewal`,`low`,`licenses`,`gap`,`notinbase`,`industry`,`tariff`];function en(e){let t=new Map;for(let n of $t)for(let r of e[n].items)for(let e of r.clients){let i=t.get(e.key)??[];i.push({topic:n,text:r.text,late:r.late}),t.set(e.key,i)}return t}var tn=`billing.expiryDays`;function nn(){try{let e=Number(localStorage.getItem(tn));return Number.isInteger(e)&&e>=1&&e<=366?e:30}catch{return 30}}function rn(e){try{localStorage.setItem(tn,String(e))}catch{}}var an={class:`view-controls`},on=[`title`],sn=[`aria-label`],cn={class:`fresh-word`},ln={key:4,class:`surface`},un={key:0,class:`meta-line`},dn=[`title`],fn=[`title`],pn={key:1},mn={class:`counters`,role:`group`,"aria-label":`Кому что сделать`},hn=[`aria-pressed`,`title`,`onClick`],gn={key:0,class:`pi pi-times`,"aria-hidden":`true`},_n={key:1,class:`filter-bar`},vn={class:`window`},yn={class:`dim`},bn={key:2,class:`filter-bar`},xn={key:0,class:`filter-tag`},Sn={key:1,class:`filter-tag`},Cn={class:`empty`},wn={class:`group-head`},Tn=[`title`,`onClick`],En={key:1},Dn={key:0,class:`stack`},On={class:`codes`},kn=[`title`,`onClick`],An=[`title`],jn={key:1,class:`dim`},Mn={class:`stack end`},Nn={key:1,class:`dim`},Pn=[`title`],Fn={key:0,class:`status`},In={key:0,class:`more-count`},Ln={key:1,class:`dim`},Rn=[`title`],zn={key:1,class:`dim`},Bn={key:0,class:`dim`},Vn={class:`dim`},Hn=le(T({__name:`ClientsView`,setup(n){let i=te(),a=ee(),u=ie(),v=Ve,T=e(null),E=e(null),D=e(nn()),{loading:O,refreshing:k,error:ne,refreshFailed:re,dataAsOf:A,load:j}=ve(async()=>{let[e,t]=await Promise.all([x.clients(),x.dashboard(D.value)]);T.value=e,E.value=t},{failText:`Не удалось загрузить клиентов.`});r(D,e=>{!Number.isInteger(e)||e<1||e>366||(rn(e),j(!0))});let M=b(()=>E.value?Qt(E.value):null),ae=b(()=>M.value?en(M.value):new Map),N=b(()=>(T.value?.clients??[]).map(e=>{let t=ae.value.get(e.key)??[],n=t[0]?$t.indexOf(t[0].topic):$t.length;return{...e,group:e.subscriberCodes[0]??``,problems:t,rank:n*100-Math.min(t.length,99)}}));function P(e){let t=i.query[e];return typeof t==`string`?t:``}function F(e,t){let n={...i.query};t?n[e]=t:delete n[e],a.replace({query:n})}let I=e(P(`q`));r(I,e=>F(`q`,e.trim()?e:``)),r(()=>i.query.q,e=>{let t=typeof e==`string`?e:``;t!==I.value&&(I.value=t)});let ce=b(()=>{let e=P(`tab`);return e===`other`||e===`all`?e:`ours`}),le=b(()=>N.value.filter(e=>e.ours).length),ue=b(()=>[{label:`Наши · ${le.value}`,value:`ours`,title:`Клиенты 1С-ЭДО с нашими идентификаторами`},{label:`Прочие · ${N.value.length-le.value}`,value:`other`,title:`Остальная база партнёра`},{label:`Все · ${N.value.length}`,value:`all`,title:`Все организации`}]),L=b(()=>ce.value===`all`?N.value:N.value.filter(e=>e.ours===(ce.value===`ours`)));function de(e){e&&F(`tab`,e===`ours`?``:e)}let R=b(()=>P(`group`)===`sub`),fe=[{label:`Без группировки`,value:`none`},{label:`По абонентам`,value:`sub`}],B=b(()=>P(`sub`)),V=Zt.filter(e=>![`invoice`,`renewal`,`anomalies`].includes(e)),me={problems:`action`,anomalies:`findings`};function ge(e){return e===`action`||e===`findings`?e:me[e]?me[e]:Zt.includes(e)&&e!==`anomalies`?e:null}let U=b(()=>ge(P(`filter`)));t(()=>{let e=P(`show`);if(!e)return;let t={...i.query};delete t.show;let n=ge(e);n&&(t.filter=n),a.replace({query:t})});function Se(e){F(`filter`,U.value===e?``:e)}function W(e){return L.value.filter(t=>t.problems.some(t=>t.topic===e)).length}let Ce=b(()=>L.value.filter(e=>e.problems.length).length),we=b(()=>E.value?.anomalies.length??0),Te=b(()=>(E.value?.anomalies??[]).filter(e=>e.item.confidence===`high`).length),De=b(()=>{let e=M.value;if(!e||!E.value)return[];let t=E.value;return[{key:`action`,label:`Требуют действий`,count:Ce.value,tone:`neutral`,hint:`Клиенты с любым поводом: счёт, лимит, продление, находки, подсказки`},{key:`invoice`,label:`Выставить счёт`,count:W(`invoice`),tone:`danger`,hint:e.invoice.items.length?`К выставлению ${K(t.billing.totalDue)} за ${q(t.billing.period)}`:`Сверх лимита за ${q(t.billing.period)} никого`},{key:`renewal`,label:`Продление ИТС`,count:W(`renewal`),tone:`warn`,hint:`Договоры 1С:ИТС, кончающиеся за ${D.value} дн.`},{key:`findings`,label:`Находки ЭДО`,count:we.value,tone:`danger`,hint:`Активных находок ${we.value}, срочных ${Te.value}`+(t.counts.reviewDue?`; скрытых пора пересмотреть: ${t.counts.reviewDue}`:``)}]}),Oe=e(null),Ae=b(()=>M.value?V.map(e=>({label:`${M.value[e].label} · ${W(e)}`,icon:U.value===e?`pi pi-check`:void 0,disabled:W(e)===0&&U.value!==e,command:()=>Se(e)})):[]),G=b(()=>U.value&&V.includes(U.value)?M.value?.[U.value]:void 0),je=b(()=>E.value?E.value.counts.drafts+E.value.counts.exported:0),Re=b(()=>{let e=I.value.trim().toLowerCase(),t=U.value,n=L.value.filter(n=>t===`action`&&!n.problems.length||t&&t!==`action`&&t!==`findings`&&!n.problems.some(e=>e.topic===t)||B.value&&!n.subscriberCodes.includes(B.value)?!1:!e||[n.clientName,n.inn,n.kpp,n.subscriberName,...n.logins,...n.edoIds,...n.subscriberCodes].join(` `).toLowerCase().includes(e));return R.value?[...n].sort((e,t)=>Number(!e.group)-Number(!t.group)||e.group.localeCompare(t.group)||e.rank-t.rank):[...n].sort((e,t)=>e.rank-t.rank||e.clientName.localeCompare(t.clientName,`ru`))}),ze=Fe(()=>Re.value,{resetOn:[I,U,B,R,ce]});r(R,e=>{e&&(ze.setSortField(void 0),ze.setSortOrder(void 0))});function He(e){return Re.value.filter(t=>t.group===e).length}let Ue=b(()=>{let e=new Map;for(let t of N.value)for(let n of t.subscriberCodes)t.subscriberName&&!e.has(n)&&e.set(n,t.subscriberName);return e}),Y={danger:`danger`,warn:`warn`,info:`info`,success:`success`};function We(e,t){return t.topic===`anomalies`?`находки ${e.anomalies||``}`.trim():M.value?.[t.topic].short??t.topic}function Ge(e){return e.problems.map(e=>`${M.value?.[e.topic].label}: ${e.text}`).join(`
`)}function Ke(e){return{over:e.overLimit,low:e.lowRemainder&&!e.overLimit}}let Je=b(()=>{if(I.value.trim())return`По запросу «${I.value.trim()}» никого не нашлось.`;if(B.value)return`У абонента ${B.value} организаций в этом списке нет.`;switch(U.value){case`action`:return`Никому ничего делать не нужно.`;case`invoice`:return`Счетов к выставлению нет.`;case`renewal`:return`Договоров 1С:ИТС, кончающихся за ${D.value} дн., нет.`;case null:case`findings`:return`Клиентов нет.`;default:return`«${M.value?.[U.value].label}»: таких клиентов нет.`}});function Ye(){I.value=``;let e={...i.query};for(let t of[`q`,`filter`,`sub`])delete e[t];a.replace({query:e})}function Xe(e){a.push({name:`request-new`,query:{inn:e.inn,kpp:e.kpp}})}let Ze=b(()=>i.name===`client`?String(i.params.key??``):``),Qe=e(``);function $e(e){a.push({name:`client`,params:{key:e},query:{...i.query,section:void 0}})}function et(){a.push({name:`clients`,query:{...i.query,section:void 0}})}function tt({data:e}){$e(e.key)}function nt(e){return e.key===Ze.value?`current`:``}let rt=b(()=>{let e=E.value?.sources;if(!e)return[];let t=E.value?.billing.period,n=Date.now()-1296e5;return[{label:t?`биллинг ${q(t)}`:`биллинг`,at:e.billing,daily:!1},{label:`база абонентов`,at:e.subscribers,daily:!0},{label:`договоры 1С:ИТС`,at:e.its,daily:!0},{label:`трафик месяца`,at:e.traffic,daily:!0},{label:`расход ЭПД`,at:e.epdUsage,daily:!0},{label:`лицензии`,at:e.licenses,daily:!0}].map(e=>({...e,stale:!e.at||e.daily&&Date.parse(e.at)<n}))}),it=b(()=>rt.value.filter(e=>e.stale).length),at=b(()=>[`Данные из 1С:`,...rt.value.map(e=>`${e.stale?`⚠ `:``}${e.label} — ${e.at?be(e.at):`ещё не было`}`)].join(`
`)),X=e(!1),ot=e(``);async function st(){X.value=!0,ot.value=``;try{await x.refreshItsContracts(),await j(!0)}catch(e){ot.value=e instanceof Error?e.message:`Не удалось проверить договоры в 1С.`}finally{X.value=!1}}let ct=e(!1),Z=e(``);async function lt(){ct.value=!0,Z.value=``;try{let e=await x.refreshSubscribers();Z.value=e.fetched?`База абонентов выгружена из 1С.`:`База выгружалась меньше 10 минут назад — показаны эти данные.`,await j(!0)}catch(e){Z.value=e instanceof Error?e.message:`Не удалось выгрузить базу абонентов.`}finally{ct.value=!1}}let dt=e(null),ft=e(!1),pt=e(``),mt=e(!1);async function ht(e){let t=e.target,n=t.files?.[0];if(t.value=``,n){ft.value=!0,pt.value=``,mt.value=!1;try{let e=await x.importEpdBilling(n);pt.value=`Выгрузка биллинга ЭПД загружена: строк ${e.epdImport.rows}, новых клиентов ${e.added}, обновлено ${e.updated}.`,await j(!0)}catch(e){mt.value=!0,pt.value=e instanceof Error?e.message:`Не удалось загрузить выгрузку биллинга.`}finally{ft.value=!1}}}let gt=b(()=>T.value?.epdImport?`Выгрузка «Детализация биллинга» ЭПД с портала 1С-ЭДО. Последняя: ${be(T.value.epdImport.importedAt)}, ${T.value.epdImport.fileName}, строк ${T.value.epdImport.rows}`:`Выгрузка «Детализация биллинга» ЭПД с портала 1С-ЭДО: её ещё не загружали`),Q=e(null),_t=e(!1);async function vt(){try{Q.value=await x.billingHistory()}catch{Q.value=null}}let yt={pending:`ещё не загружен`,none:`в 1С биллинга нет`,failed:`1С не отдала отчёт`},bt=b(()=>(Q.value?.months??[]).filter(e=>e.status===`ok`)),xt=b(()=>(Q.value?.months??[]).map(e=>({key:e.period,label:q(e.period),value:e.status===`ok`?e.packets:null,over:e.billable>0,note:e.status===`ok`?`сверх лимита ${e.billable}, к выставлению ${K(e.totalDue)}`:yt[e.status]}))),St=b(()=>{let e=bt.value.reduce((e,t)=>e+(Ee(t.totalDue)||0),0);return K(e.toFixed(2).replace(`.`,`,`))});function Ct(){_t.value=!0,vt()}let wt=e(null),Tt=b(()=>[{label:`Расход по месяцам`,icon:`pi pi-chart-bar`,command:Ct},{label:ct.value?`Выгружаем базу абонентов…`:`Выгрузить базу абонентов из 1С`,icon:`pi pi-download`,disabled:ct.value,command:lt},{label:X.value?`Проверяем договоры…`:`Проверить договоры 1С:ИТС в 1С`,icon:`pi pi-refresh`,disabled:X.value,command:st}]),Et=e(null);function Dt(){j(!0),Et.value?.reload(),_t.value&&vt()}function Ot(){j(!0),Et.value?.reload()}return r(()=>u.lastEvent,()=>{_t.value&&vt()}),(e,t)=>(o(),C(`section`,null,[m(Le,{search:I.value,"onUpdate:search":t[3]||=e=>I.value=e,placeholder:`Клиент, ИНН, логин, абонент, ID`,refreshing:y(k),"data-as-of":y(A),stamp:``,"refresh-failed":y(re),error:y(ne),onRefresh:Dt},{meta:f(()=>[w(`div`,an,[m(y(Nt),{"model-value":ce.value,options:ue.value,"option-label":`label`,"option-value":`value`,"allow-empty":!1,"aria-label":`Чьи клиенты`,class:`scope`,"onUpdate:modelValue":de},{option:f(({option:e})=>[w(`span`,{title:e.title},d(e.label),9,on)]),_:1},8,[`model-value`,`options`]),m(y(Be),{"model-value":R.value?`sub`:`none`,options:fe,"option-label":`label`,"option-value":`value`,"aria-label":`Группировка`,class:`grouping`,"onUpdate:modelValue":t[0]||=e=>F(`group`,e===`sub`?`sub`:``)},null,8,[`model-value`])])]),tools:f(()=>[y(A)||rt.value.length?p((o(),C(`span`,{key:0,class:l([`freshness`,{stale:it.value}]),tabindex:`0`,"aria-label":at.value},[w(`span`,cn,d(y(k)?`Обновляем…`:`Обновлено в`),1),y(k)?g(``,!0):(o(),C(S,{key:0},[_(d(y(A)),1)],64)),w(`i`,{class:l(it.value?`pi pi-exclamation-circle`:`pi pi-info-circle`)},null,2)],10,sn)),[[y(v),at.value,void 0,{bottom:!0}]]):g(``,!0),m(y(z),{label:`Загрузить CSV`,icon:`pi pi-upload`,size:`small`,class:`import`,loading:ft.value,title:gt.value,onClick:t[1]||=e=>dt.value?.click()},null,8,[`loading`,`title`]),w(`input`,{ref_key:`importInput`,ref:dt,type:`file`,accept:`.csv,text/csv`,hidden:``,"data-testid":`epd-import-file`,onChange:ht},null,544),m(y(z),{icon:`pi pi-ellipsis-h`,size:`small`,outlined:``,"aria-label":`Ещё действия`,"aria-haspopup":`true`,"aria-controls":`clients-actions`,onClick:t[2]||=e=>wt.value?.toggle(e)}),m(y(ut),{id:`clients-actions`,ref_key:`actionsMenu`,ref:wt,model:Tt.value,popup:``},null,8,[`model`])]),_:1},8,[`search`,`refreshing`,`data-as-of`,`refresh-failed`,`error`]),Z.value?(o(),h(y(H),{key:0,severity:`info`,closable:!1},{default:f(()=>[_(d(Z.value),1)]),_:1})):g(``,!0),pt.value?(o(),h(y(H),{key:1,severity:mt.value?`error`:`success`,closable:!1},{default:f(()=>[_(d(pt.value),1)]),_:1},8,[`severity`])):g(``,!0),ot.value?(o(),h(y(H),{key:2,severity:`error`,closable:!1},{default:f(()=>[_(d(ot.value),1)]),_:1})):g(``,!0),T.value&&!T.value.subscribersFetchedAt?(o(),h(y(H),{key:3,severity:`info`,closable:!1},{default:f(()=>[...t[13]||=[_(` Базу абонентов ещё не выгружали: она обновляется раз в сутки, или «⋯» → «Выгрузить базу абонентов из 1С». `,-1)]]),_:1})):g(``,!0),y(ne)?g(``,!0):(o(),C(`div`,ln,[T.value?(o(),C(`p`,un,[_(d(Re.value.length)+` из `+d(L.value.length)+` `+d(y(Me)(L.value.length,`клиента`,`клиентов`,`клиентов`))+` · `,1),w(`span`,{title:T.value.takenAt?`Снимок биллинга от ${y(be)(T.value.takenAt)}`:``},`биллинг `+d(y(q)(T.value.period)),9,dn),t[14]||=_(` · `,-1),T.value.epdImport?(o(),C(`span`,{key:0,title:`${T.value.epdImport.fileName}, строк ${T.value.epdImport.rows}`},`CSV ЭПД загружен `+d(y(be)(T.value.epdImport.importedAt)),9,fn)):(o(),C(`span`,pn,`CSV ЭПД не загружали`))])):g(``,!0),w(`div`,mn,[(o(!0),C(S,null,s(De.value,e=>(o(),C(`button`,{key:e.key,type:`button`,class:l([`counter`,[e.tone,{active:U.value===e.key,zero:!e.count}]]),"aria-pressed":U.value===e.key,title:e.hint,onClick:t=>Se(e.key)},[w(`strong`,null,d(e.count),1),w(`span`,null,d(e.label),1),U.value===e.key?(o(),C(`i`,gn)):g(``,!0)],10,hn))),128)),m(y(z),{label:G.value?`${G.value.label} · ${W(G.value.key)}`:`Ещё фильтры`,icon:G.value?`pi pi-filter-fill`:`pi pi-filter`,"icon-pos":`left`,size:`small`,outlined:!G.value,class:`more`,"aria-haspopup":`true`,"aria-controls":`clients-more`,onClick:t[4]||=e=>Oe.value?.toggle(e)},null,8,[`label`,`icon`,`outlined`]),m(y(ut),{id:`clients-more`,ref_key:`moreMenu`,ref:Oe,model:Ae.value,popup:``},null,8,[`model`]),je.value?(o(),h(y(pe),{key:0,to:{name:`requests`},class:`counter info link`,title:`черновиков ${E.value?.counts.drafts??0} · выгружено файлом ${E.value?.counts.exported??0}`},{default:f(()=>[w(`strong`,null,d(je.value),1),t[15]||=w(`span`,null,`Неотправленные заявки`,-1),t[16]||=w(`i`,{class:`pi pi-arrow-right`,"aria-hidden":`true`},null,-1)]),_:1},8,[`title`])):g(``,!0)]),U.value===`renewal`?(o(),C(`div`,_n,[w(`label`,vn,[t[17]||=_(` Договоры и лицензии, кончающиеся за `,-1),m(y(Ie),{modelValue:D.value,"onUpdate:modelValue":t[5]||=e=>D.value=e,min:1,max:366,"use-grouping":!1,"input-class":`days`,"aria-label":`Окно напоминаний, дней`},null,8,[`modelValue`]),t[18]||=_(` дн. `,-1)]),m(y(z),{label:`Проверить в 1С`,icon:`pi pi-refresh`,size:`small`,outlined:``,loading:X.value,onClick:st},null,8,[`loading`]),w(`small`,yn,d(E.value?.sources.its?`проверено ${y(be)(E.value.sources.its)}`:`договоры ещё не проверялись`),1)])):g(``,!0),B.value||G.value?(o(),C(`div`,bn,[G.value?(o(),C(`span`,xn,[_(d(G.value.label)+` `,1),m(y(z),{icon:`pi pi-times`,size:`small`,text:``,"aria-label":`Снять фильтр «${G.value.label}»`,onClick:t[6]||=e=>Se(G.value.key)},null,8,[`aria-label`])])):g(``,!0),B.value?(o(),C(`span`,Sn,[t[19]||=_(` Абонент `,-1),w(`code`,null,d(B.value),1),Ue.value.get(B.value)?(o(),C(S,{key:0},[_(` — `+d(Ue.value.get(B.value)),1)],64)):g(``,!0),m(y(z),{icon:`pi pi-times`,size:`small`,text:``,"aria-label":`Снять фильтр по абоненту`,onClick:t[7]||=e=>F(`sub`,``)})])):g(``,!0)])):g(``,!0),U.value===`findings`?(o(),h(Xt,{key:3,ref_key:`findingsPanel`,ref:Et,search:I.value,onClearSearch:t[8]||=e=>I.value=``,onChanged:t[9]||=e=>y(j)(!0),onOpen:$e},null,8,[`search`])):(o(),h(Ne,{key:4,feed:y(ze),loading:y(O),"data-key":`key`,"row-group-mode":R.value?`subheader`:void 0,"group-rows-by":`group`,"row-class":nt,class:`clients`,onRowClick:tt},{empty:f(()=>[w(`div`,Cn,[w(`span`,null,d(Je.value),1),I.value.trim()||U.value||B.value?(o(),h(y(z),{key:0,label:`Показать всех`,size:`small`,text:``,onClick:Ye})):g(``,!0)])]),groupheader:f(({data:e})=>[w(`div`,wn,[e.group?(o(),C(S,{key:0},[w(`button`,{type:`button`,class:`linkish`,title:`Только абонент ${e.group}`,onClick:se(t=>F(`sub`,e.group),[`stop`])},[w(`code`,null,d(e.group),1)],8,Tn),w(`span`,null,d(Ue.value.get(e.group)||`нет в базе абонентов`),1)],64)):(o(),C(`span`,En,`Абонент не известен`)),w(`small`,null,d(He(e.group))+` `+d(y(Me)(He(e.group),`организация`,`организации`,`организаций`)),1)])]),default:f(()=>[m(y(J),{field:`clientName`,header:`Клиент`,sortable:!R.value,style:{"min-width":`15rem`}},{body:f(({data:e})=>[m(Pe,{name:e.clientName,inn:e.inn,kpp:e.kpp},null,8,[`name`,`inn`,`kpp`])]),_:1},8,[`sortable`]),m(y(J),{field:`group`,header:`Абонент`,sortable:!R.value,class:`wide-only`},{body:f(({data:e})=>[e.subscriberCodes.length?(o(),C(`div`,Dn,[w(`span`,On,[(o(!0),C(S,null,s(e.subscriberCodes,e=>(o(),C(`button`,{key:e,type:`button`,class:`linkish`,title:`Все организации абонента ${e}`,onClick:se(t=>F(`sub`,e),[`stop`])},[w(`code`,null,d(e),1)],8,kn))),128))]),e.subscriberName?(o(),C(`small`,{key:0,class:`ellipsis`,title:e.subscriberName},d(e.subscriberName),9,An)):g(``,!0)])):(o(),C(`span`,jn,`—`))]),_:1},8,[`sortable`]),m(y(J),{field:`used`,header:`Пакеты / лимит`,sortable:!R.value,class:`num`,style:{width:`10.5rem`}},{body:f(({data:e})=>[w(`div`,Mn,[e.inBilling?(o(),C(`span`,{key:0,class:l([`usage`,Ke(e)])},[_(d(e.used)+` `,1),w(`small`,null,d(e.limit===null?`без лимита`:`из ${e.limit}`),1)],2)):(o(),C(`span`,Nn,`—`)),e.tariff?(o(),C(`small`,{key:2,class:`ellipsis tariff`,title:`Тариф: ${e.tariff}`},d(e.tariff),9,Pn)):g(``,!0)])]),_:1},8,[`sortable`]),m(y(J),{field:`billable`,header:`Счёт`,sortable:!R.value,class:`num wide-only`,style:{width:`8rem`}},{body:f(({data:e})=>[w(`span`,{class:l({dim:e.billable<=0})},d(e.billable>0?y(K)(e.amount):`—`),3)]),_:1},8,[`sortable`]),m(y(J),{field:`rank`,header:`Статус`,sortable:!R.value,style:{width:`10rem`}},{body:f(({data:e})=>[e.problems.length?p((o(),C(`span`,Fn,[m(y(_e),{severity:Y[M.value[e.problems[0].topic].tone],value:We(e,e.problems[0])},null,8,[`severity`,`value`]),e.problems.length>1?(o(),C(`small`,In,`+`+d(e.problems.length-1),1)):g(``,!0)])),[[y(v),Ge(e),void 0,{left:!0}]]):(o(),C(`span`,Ln,`—`))]),_:1},8,[`sortable`]),m(y(J),{field:`lastRequestAt`,header:`Последняя заявка`,sortable:!R.value,class:`wide-only nowrap`,style:{width:`10rem`}},{body:f(({data:e})=>[e.requests?(o(),C(`span`,{key:0,title:`Заявок на клиента: ${e.requests}`},d(e.requests)+` · `+d(y(xe)(e.lastRequestAt)),9,Rn)):(o(),C(`span`,zn,`—`))]),_:1},8,[`sortable`]),m(y(J),{header:``,class:`row-action`,style:{width:`6.5rem`}},{body:f(({data:e})=>[m(y(z),{label:`Заявка`,icon:`pi pi-plus`,size:`small`,text:``,disabled:!y(ke)(e.inn),title:y(ke)(e.inn)?`Новая заявка на ${e.clientName}`:`ИНН в 1С не заполнен: заявку не завести`,onClick:se(t=>Xe(e),[`stop`])},null,8,[`disabled`,`title`,`onClick`])]),_:1})]),_:1},8,[`feed`,`loading`,`row-group-mode`]))])),m(y(qe),{visible:!!Ze.value,position:`right`,header:Qe.value||`Карточка клиента`,class:`client-drawer`,style:{width:`min(760px, 100vw)`},"onUpdate:visible":t[11]||=e=>{e||et()}},{default:f(()=>[Ze.value?(o(),h(y(he),{key:0},{default:f(({Component:e})=>[(o(),h(c(e),{onChanged:Ot,onTitle:t[10]||=e=>Qe.value=e},null,32))]),_:1})):g(``,!0)]),_:1},8,[`visible`,`header`]),m(y(oe),{visible:_t.value,"onUpdate:visible":t[12]||=e=>_t.value=e,modal:``,header:`Расход по месяцам, пакетов, все клиенты`,style:{width:`min(960px, calc(100vw - 32px))`}},{default:f(()=>[Q.value?(o(),C(S,{key:1},[w(`p`,Vn,[_(` Загружено `+d(bt.value.length)+` из `+d(Q.value.months.length)+` мес. `,1),bt.value.length?(o(),C(S,{key:0},[_(` · выставлено за загруженные месяцы `+d(St.value),1)],64)):g(``,!0)]),m(ye,{bars:xt.value,unit:`пакетов`,title:`Пакеты документов ЭДО по месяцам, все клиенты`},null,8,[`bars`])],64)):(o(),C(`p`,Bn,` Загружаем историю биллинга… `))]),_:1},8,[`visible`])]))}}),[[`__scopeId`,`data-v-e4f95dc2`]]);export{Hn as default};