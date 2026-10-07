import{$ as e,F as t,H as n,L as r,M as i,P as a,R as o,St as s,T as c,Tt as l,U as u,W as d,_ as f,d as p,f as m,g as h,it as g,l as _,n as v,o as y,p as b,u as x,v as S,wt as C}from"./client-BuiZRfLx.js";import{D as w,K as T,P as E,V as D,W as ee,c as O,et as te,i as ne,l as k,lt as A,m as j,n as M,p as N,st as P,ut as F,v as I,vt as re,z as L}from"./index-BjOK8fFh.js";import{i as ie,t as ae}from"./message-BTBQ3XbE.js";import{r as R}from"./useRefreshable-CFGNYu72.js";import{t as z}from"./textarea--B1hAro0.js";var B=I.extend({name:`tab`,classes:{root:function(e){var t=e.instance,n=e.props;return[`p-tab`,{"p-tab-active":t.active,"p-disabled":n.disabled}]}}}),V={name:`Tab`,extends:{name:`BaseTab`,extends:j,props:{value:{type:[String,Number],default:void 0},disabled:{type:Boolean,default:!1},as:{type:[String,Object],default:`BUTTON`},asChild:{type:Boolean,default:!1}},style:B,provide:function(){return{$pcTab:this,$parentInstance:this}}},inheritAttrs:!1,inject:[`$pcTabs`,`$pcTabList`],methods:{onFocus:function(){this.$pcTabs.selectOnFocus&&this.changeActiveValue()},onClick:function(){this.changeActiveValue()},onKeydown:function(e){switch(e.code){case`ArrowRight`:this.onArrowRightKey(e);break;case`ArrowLeft`:this.onArrowLeftKey(e);break;case`Home`:this.onHomeKey(e);break;case`End`:this.onEndKey(e);break;case`PageDown`:this.onPageDownKey(e);break;case`PageUp`:this.onPageUpKey(e);break;case`Enter`:case`NumpadEnter`:case`Space`:this.onEnterKey(e)}},onArrowRightKey:function(e){var t=this.findNextTab(e.currentTarget);t?this.changeFocusedTab(e,t):this.onHomeKey(e),e.preventDefault()},onArrowLeftKey:function(e){var t=this.findPrevTab(e.currentTarget);t?this.changeFocusedTab(e,t):this.onEndKey(e),e.preventDefault()},onHomeKey:function(e){var t=this.findFirstTab();this.changeFocusedTab(e,t),e.preventDefault()},onEndKey:function(e){var t=this.findLastTab();this.changeFocusedTab(e,t),e.preventDefault()},onPageDownKey:function(e){this.scrollInView(this.findLastTab()),e.preventDefault()},onPageUpKey:function(e){this.scrollInView(this.findFirstTab()),e.preventDefault()},onEnterKey:function(e){this.changeActiveValue()},findNextTab:function(e){var t=arguments.length>1&&arguments[1]!==void 0&&arguments[1]?e:e.nextElementSibling;return t?L(t,`data-p-disabled`)||L(t,`data-pc-section`)===`activebar`?this.findNextTab(t):A(t,`[data-pc-name="tab"]`):null},findPrevTab:function(e){var t=arguments.length>1&&arguments[1]!==void 0&&arguments[1]?e:e.previousElementSibling;return t?L(t,`data-p-disabled`)||L(t,`data-pc-section`)===`activebar`?this.findPrevTab(t):A(t,`[data-pc-name="tab"]`):null},findFirstTab:function(){return this.findNextTab(this.$pcTabList.$refs.tabs.firstElementChild,!0)},findLastTab:function(){return this.findPrevTab(this.$pcTabList.$refs.tabs.lastElementChild,!0)},changeActiveValue:function(){this.$pcTabs.updateValue(this.value)},changeFocusedTab:function(e,t){te(t),this.scrollInView(t)},scrollInView:function(e){var t;e==null||(t=e.scrollIntoView)==null||t.call(e,{block:`nearest`})}},computed:{active:function(){return re(this.$pcTabs?.d_value,this.value)},id:function(){return`${this.$pcTabs?.$id}_tab_${this.value}`},ariaControls:function(){return`${this.$pcTabs?.$id}_tabpanel_${this.value}`},attrs:function(){return c(this.asAttrs,this.a11yAttrs,this.ptmi(`root`,this.ptParams))},asAttrs:function(){return this.as===`BUTTON`?{type:`button`,disabled:this.disabled}:void 0},a11yAttrs:function(){return{id:this.id,tabindex:this.active?this.$pcTabs.tabindex:-1,role:`tab`,"aria-selected":this.active,"aria-controls":this.ariaControls,"data-pc-name":`tab`,"data-p-disabled":this.disabled,"data-p-active":this.active,onFocus:this.onFocus,onKeydown:this.onKeydown}},ptParams:function(){return{context:{active:this.active}}},dataP:function(){return F({active:this.active})}},directives:{ripple:k}};function H(e,n,a,l,f,m){var h=r(`ripple`);return e.asChild?t(e.$slots,`default`,{key:1,dataP:m.dataP,class:s(e.cx(`root`)),active:m.active,a11yAttrs:m.a11yAttrs,onClick:m.onClick}):d((i(),p(o(e.as),c({key:0,class:e.cx(`root`),"data-p":m.dataP,onClick:m.onClick},m.attrs),{default:u(function(){return[t(e.$slots,`default`)]}),_:3},16,[`class`,`data-p`,`onClick`])),[[h]])}V.render=H;var U={name:`ChevronLeftIcon`,extends:N};function W(e){return J(e)||q(e)||K(e)||G()}function G(){throw TypeError(`Invalid attempt to spread non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`)}function K(e,t){if(e){if(typeof e==`string`)return Y(e,t);var n={}.toString.call(e).slice(8,-1);return n===`Object`&&e.constructor&&(n=e.constructor.name),n===`Map`||n===`Set`?Array.from(e):n===`Arguments`||/^(?:Ui|I)nt(?:8|16|32)(?:Clamped)?Array$/.test(n)?Y(e,t):void 0}}function q(e){if(typeof Symbol<`u`&&e[Symbol.iterator]!=null||e[`@@iterator`]!=null)return Array.from(e)}function J(e){if(Array.isArray(e))return Y(e)}function Y(e,t){(t==null||t>e.length)&&(t=e.length);for(var n=0,r=Array(t);n<t;n++)r[n]=e[n];return r}function oe(e,t,n,r,a,o){return i(),b(`svg`,c({width:`14`,height:`14`,viewBox:`0 0 14 14`,fill:`none`,xmlns:`http://www.w3.org/2000/svg`},e.pti()),W(t[0]||=[x(`path`,{d:`M9.61296 13C9.50997 13.0005 9.40792 12.9804 9.3128 12.9409C9.21767 12.9014 9.13139 12.8433 9.05902 12.7701L3.83313 7.54416C3.68634 7.39718 3.60388 7.19795 3.60388 6.99022C3.60388 6.78249 3.68634 6.58325 3.83313 6.43628L9.05902 1.21039C9.20762 1.07192 9.40416 0.996539 9.60724 1.00012C9.81032 1.00371 10.0041 1.08597 10.1477 1.22959C10.2913 1.37322 10.3736 1.56698 10.3772 1.77005C10.3808 1.97313 10.3054 2.16968 10.1669 2.31827L5.49496 6.99022L10.1669 11.6622C10.3137 11.8091 10.3962 12.0084 10.3962 12.2161C10.3962 12.4238 10.3137 12.6231 10.1669 12.7701C10.0945 12.8433 10.0083 12.9014 9.91313 12.9409C9.81801 12.9804 9.71596 13.0005 9.61296 13Z`,fill:`currentColor`},null,-1)]),16)}U.render=oe;var se=I.extend({name:`tablist`,classes:{root:`p-tablist`,content:`p-tablist-content p-tablist-viewport`,tabList:`p-tablist-tab-list`,activeBar:`p-tablist-active-bar`,prevButton:`p-tablist-prev-button p-tablist-nav-button`,nextButton:`p-tablist-next-button p-tablist-nav-button`}}),X={name:`TabList`,extends:{name:`BaseTabList`,extends:j,props:{},style:se,provide:function(){return{$pcTabList:this,$parentInstance:this}}},inheritAttrs:!1,inject:[`$pcTabs`],data:function(){return{isPrevButtonEnabled:!1,isNextButtonEnabled:!0}},resizeObserver:void 0,inkBarObserver:void 0,watch:{showNavigators:function(e){e?this.bindResizeObserver():this.unbindResizeObserver()},activeValue:{flush:`post`,handler:function(){this.updateInkBar(),this.bindInkBarObserver()}}},mounted:function(){var e=this;setTimeout(function(){e.updateInkBar(),e.bindInkBarObserver()},150),this.showNavigators&&(this.updateButtonState(),this.bindResizeObserver())},updated:function(){this.showNavigators&&this.updateButtonState()},beforeUnmount:function(){this.unbindResizeObserver(),this.unbindInkBarObserver()},methods:{onScroll:function(e){this.showNavigators&&this.updateButtonState(),e.preventDefault()},onPrevButtonClick:function(){var e=this.$refs.content,t=this.getVisibleButtonWidths(),n=D(e)-t,r=Math.abs(e.scrollLeft)-n*.8,i=Math.max(r,0);e.scrollLeft=T(e)?-1*i:i},onNextButtonClick:function(){var e=this.$refs.content,t=this.getVisibleButtonWidths(),n=D(e)-t,r=Math.abs(e.scrollLeft)+n*.8,i=e.scrollWidth-n,a=Math.min(r,i);e.scrollLeft=T(e)?-1*a:a},bindResizeObserver:function(){var e=this;this.resizeObserver=new ResizeObserver(function(){return e.updateButtonState()}),this.resizeObserver.observe(this.$refs.list)},unbindResizeObserver:function(){var e;(e=this.resizeObserver)==null||e.unobserve(this.$refs.list),this.resizeObserver=void 0},bindInkBarObserver:function(){var e=this;this.unbindInkBarObserver();var t=this.$refs.content,n=A(t,`[data-pc-name="tab"][data-p-active="true"]`);n&&(this.inkBarObserver=new ResizeObserver(function(){return e.updateInkBar()}),this.inkBarObserver.observe(n))},unbindInkBarObserver:function(){var e;(e=this.inkBarObserver)==null||e.disconnect(),this.inkBarObserver=void 0},updateInkBar:function(){var e=this.$refs,t=e.content,n=e.inkbar,r=e.tabs;if(n){var i=A(t,`[data-pc-name="tab"][data-p-active="true"]`);this.$pcTabs.isVertical()?(n.style.height=w(i)+`px`,n.style.top=E(i).top-E(r).top+`px`):(n.style.width=P(i)+`px`,n.style.left=E(i).left-E(r).left+`px`)}},updateButtonState:function(){var e=this.$refs,t=e.list,n=e.content,r=n.scrollTop,i=n.scrollWidth,a=n.scrollHeight,o=n.offsetWidth,s=n.offsetHeight,c=Math.abs(n.scrollLeft),l=[D(n),ee(n)],u=l[0],d=l[1];this.$pcTabs.isVertical()?(this.isPrevButtonEnabled=r!==0,this.isNextButtonEnabled=t.offsetHeight>=s&&parseInt(r)!==a-d):(this.isPrevButtonEnabled=c!==0,this.isNextButtonEnabled=t.offsetWidth>=o&&parseInt(c)!==i-u)},getVisibleButtonWidths:function(){var e=this.$refs,t=e.prevButton,n=e.nextButton,r=0;return this.showNavigators&&(r=(t?.offsetWidth||0)+(n?.offsetWidth||0)),r}},computed:{templates:function(){return this.$pcTabs.$slots},activeValue:function(){return this.$pcTabs.d_value},showNavigators:function(){return this.$pcTabs.showNavigators},prevButtonAriaLabel:function(){return this.$primevue.config.locale.aria?this.$primevue.config.locale.aria.previous:void 0},nextButtonAriaLabel:function(){return this.$primevue.config.locale.aria?this.$primevue.config.locale.aria.next:void 0},dataP:function(){return F({scrollable:this.$pcTabs.scrollable})}},components:{ChevronLeftIcon:U,ChevronRightIcon:R},directives:{ripple:k}},ce=[`data-p`],le=[`aria-label`,`tabindex`],ue=[`data-p`],de=[`aria-orientation`],fe=[`aria-label`,`tabindex`];function pe(e,n,a,s,l,u){var f=r(`ripple`);return i(),b(`div`,c({ref:`list`,class:e.cx(`root`),"data-p":u.dataP},e.ptmi(`root`)),[u.showNavigators&&l.isPrevButtonEnabled?d((i(),b(`button`,c({key:0,ref:`prevButton`,type:`button`,class:e.cx(`prevButton`),"aria-label":u.prevButtonAriaLabel,tabindex:u.$pcTabs.tabindex,onClick:n[0]||=function(){return u.onPrevButtonClick&&u.onPrevButtonClick.apply(u,arguments)}},e.ptm(`prevButton`),{"data-pc-group-section":`navigator`}),[(i(),p(o(u.templates.previcon||`ChevronLeftIcon`),c({"aria-hidden":`true`},e.ptm(`prevIcon`)),null,16))],16,le)),[[f]]):m(``,!0),x(`div`,c({ref:`content`,class:e.cx(`content`),onScroll:n[1]||=function(){return u.onScroll&&u.onScroll.apply(u,arguments)},"data-p":u.dataP},e.ptm(`content`)),[x(`div`,c({ref:`tabs`,class:e.cx(`tabList`),role:`tablist`,"aria-orientation":u.$pcTabs.orientation||`horizontal`},e.ptm(`tabList`)),[t(e.$slots,`default`),x(`span`,c({ref:`inkbar`,class:e.cx(`activeBar`),role:`presentation`,"aria-hidden":`true`},e.ptm(`activeBar`)),null,16)],16,de)],16,ue),u.showNavigators&&l.isNextButtonEnabled?d((i(),b(`button`,c({key:1,ref:`nextButton`,type:`button`,class:e.cx(`nextButton`),"aria-label":u.nextButtonAriaLabel,tabindex:u.$pcTabs.tabindex,onClick:n[2]||=function(){return u.onNextButtonClick&&u.onNextButtonClick.apply(u,arguments)}},e.ptm(`nextButton`),{"data-pc-group-section":`navigator`}),[(i(),p(o(u.templates.nexticon||`ChevronRightIcon`),c({"aria-hidden":`true`},e.ptm(`nextIcon`)),null,16))],16,fe)),[[f]]):m(``,!0)],16,ce)}X.render=pe;var me=I.extend({name:`tabs`,style:`
    .p-tabs {
        display: flex;
        flex-direction: column;
    }

    .p-tablist {
        display: flex;
        position: relative;
        overflow: hidden;
        background: dt('tabs.tablist.background');
    }

    .p-tablist-viewport {
        overflow-x: auto;
        overflow-y: hidden;
        scroll-behavior: smooth;
        scrollbar-width: none;
        overscroll-behavior: contain auto;
    }

    .p-tablist-viewport::-webkit-scrollbar {
        display: none;
    }

    .p-tablist-tab-list {
        position: relative;
        display: flex;
        border-style: solid;
        border-color: dt('tabs.tablist.border.color');
        border-width: dt('tabs.tablist.border.width');
    }

    .p-tablist-content {
        flex-grow: 1;
    }

    .p-tablist-nav-button {
        all: unset;
        position: absolute !important;
        flex-shrink: 0;
        inset-block-start: 0;
        z-index: 2;
        height: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
        background: dt('tabs.nav.button.background');
        color: dt('tabs.nav.button.color');
        width: dt('tabs.nav.button.width');
        transition:
            color dt('tabs.transition.duration'),
            outline-color dt('tabs.transition.duration'),
            box-shadow dt('tabs.transition.duration');
        box-shadow: dt('tabs.nav.button.shadow');
        outline-color: transparent;
        cursor: pointer;
    }

    .p-tablist-nav-button:focus-visible {
        z-index: 1;
        box-shadow: dt('tabs.nav.button.focus.ring.shadow');
        outline: dt('tabs.nav.button.focus.ring.width') dt('tabs.nav.button.focus.ring.style') dt('tabs.nav.button.focus.ring.color');
        outline-offset: dt('tabs.nav.button.focus.ring.offset');
    }

    .p-tablist-nav-button:hover {
        color: dt('tabs.nav.button.hover.color');
    }

    .p-tablist-prev-button {
        inset-inline-start: 0;
    }

    .p-tablist-next-button {
        inset-inline-end: 0;
    }

    .p-tablist-prev-button:dir(rtl),
    .p-tablist-next-button:dir(rtl) {
        transform: rotate(180deg);
    }

    .p-tab {
        flex-shrink: 0;
        cursor: pointer;
        user-select: none;
        position: relative;
        border-style: solid;
        white-space: nowrap;
        gap: dt('tabs.tab.gap');
        background: dt('tabs.tab.background');
        border-width: dt('tabs.tab.border.width');
        border-color: dt('tabs.tab.border.color');
        color: dt('tabs.tab.color');
        padding: dt('tabs.tab.padding');
        font-weight: dt('tabs.tab.font.weight');
        transition:
            background dt('tabs.transition.duration'),
            border-color dt('tabs.transition.duration'),
            color dt('tabs.transition.duration'),
            outline-color dt('tabs.transition.duration'),
            box-shadow dt('tabs.transition.duration');
        margin: dt('tabs.tab.margin');
        outline-color: transparent;
    }

    .p-tab:not(.p-disabled):focus-visible {
        z-index: 1;
        box-shadow: dt('tabs.tab.focus.ring.shadow');
        outline: dt('tabs.tab.focus.ring.width') dt('tabs.tab.focus.ring.style') dt('tabs.tab.focus.ring.color');
        outline-offset: dt('tabs.tab.focus.ring.offset');
    }

    .p-tab:not(.p-tab-active):not(.p-disabled):hover {
        background: dt('tabs.tab.hover.background');
        border-color: dt('tabs.tab.hover.border.color');
        color: dt('tabs.tab.hover.color');
    }

    .p-tab-active {
        background: dt('tabs.tab.active.background');
        border-color: dt('tabs.tab.active.border.color');
        color: dt('tabs.tab.active.color');
    }

    .p-tabpanels {
        background: dt('tabs.tabpanel.background');
        color: dt('tabs.tabpanel.color');
        padding: dt('tabs.tabpanel.padding');
        outline: 0 none;
    }

    .p-tabpanel:focus-visible {
        box-shadow: dt('tabs.tabpanel.focus.ring.shadow');
        outline: dt('tabs.tabpanel.focus.ring.width') dt('tabs.tabpanel.focus.ring.style') dt('tabs.tabpanel.focus.ring.color');
        outline-offset: dt('tabs.tabpanel.focus.ring.offset');
    }

    .p-tablist-active-bar {
        z-index: 1;
        display: block;
        position: absolute;
        inset-block-end: dt('tabs.active.bar.bottom');
        height: dt('tabs.active.bar.height');
        background: dt('tabs.active.bar.background');
        transition: 250ms cubic-bezier(0.35, 0, 0.25, 1);
    }
`,classes:{root:function(e){return[`p-tabs p-component`,{"p-tabs-scrollable":e.props.scrollable}]}}}),Z={name:`Tabs`,extends:{name:`BaseTabs`,extends:j,props:{value:{type:[String,Number],default:void 0},lazy:{type:Boolean,default:!1},scrollable:{type:Boolean,default:!1},showNavigators:{type:Boolean,default:!0},tabindex:{type:Number,default:0},selectOnFocus:{type:Boolean,default:!1}},style:me,provide:function(){return{$pcTabs:this,$parentInstance:this}}},inheritAttrs:!1,emits:[`update:value`],data:function(){return{d_value:this.value}},watch:{value:function(e){this.d_value=e}},methods:{updateValue:function(e){this.d_value!==e&&(this.d_value=e,this.$emit(`update:value`,e))},isVertical:function(){return this.orientation===`vertical`}}};function he(e,n,r,a,o,s){return i(),b(`div`,c({class:e.cx(`root`)},e.ptmi(`root`)),[t(e.$slots,`default`)],16)}Z.render=he;var ge=I.extend({name:`toggleswitch`,style:`
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
`,classes:{root:function(e){var t=e.instance,n=e.props;return[`p-toggleswitch p-component`,{"p-toggleswitch-checked":t.checked,"p-disabled":n.disabled,"p-invalid":t.$invalid}]},input:`p-toggleswitch-input`,slider:`p-toggleswitch-slider`,handle:`p-toggleswitch-handle`},inlineStyles:{root:{position:`relative`}}}),Q={name:`ToggleSwitch`,extends:{name:`BaseToggleSwitch`,extends:ie,props:{trueValue:{type:null,default:!0},falseValue:{type:null,default:!1},readonly:{type:Boolean,default:!1},tabindex:{type:Number,default:null},inputId:{type:String,default:null},inputClass:{type:[String,Object],default:null},inputStyle:{type:Object,default:null},ariaLabelledby:{type:String,default:null},ariaLabel:{type:String,default:null}},style:ge,provide:function(){return{$pcToggleSwitch:this,$parentInstance:this}}},inheritAttrs:!1,emits:[`change`,`focus`,`blur`],methods:{getPTOptions:function(e){return(e===`root`?this.ptmi:this.ptm)(e,{context:{checked:this.checked,disabled:this.disabled}})},onChange:function(e){if(!this.disabled&&!this.readonly){var t=this.checked?this.falseValue:this.trueValue;this.writeValue(t,e),this.$emit(`change`,e)}},onFocus:function(e){this.$emit(`focus`,e)},onBlur:function(e){var t,n;this.$emit(`blur`,e),(t=(n=this.formField).onBlur)==null||t.call(n,e)}},computed:{checked:function(){return this.d_value===this.trueValue},dataP:function(){return F({checked:this.checked,disabled:this.disabled,invalid:this.$invalid})}}},_e=[`data-p-checked`,`data-p-disabled`,`data-p`],ve=[`id`,`checked`,`tabindex`,`disabled`,`readonly`,`aria-checked`,`aria-labelledby`,`aria-label`,`aria-invalid`],ye=[`data-p`],be=[`data-p`];function $(e,n,r,a,o,s){return i(),b(`div`,c({class:e.cx(`root`),style:e.sx(`root`)},s.getPTOptions(`root`),{"data-p-checked":s.checked,"data-p-disabled":e.disabled,"data-p":s.dataP}),[x(`input`,c({id:e.inputId,type:`checkbox`,role:`switch`,class:[e.cx(`input`),e.inputClass],style:e.inputStyle,checked:s.checked,tabindex:e.tabindex,disabled:e.disabled,readonly:e.readonly,"aria-checked":s.checked,"aria-labelledby":e.ariaLabelledby,"aria-label":e.ariaLabel,"aria-invalid":e.invalid||void 0,onFocus:n[0]||=function(){return s.onFocus&&s.onFocus.apply(s,arguments)},onBlur:n[1]||=function(){return s.onBlur&&s.onBlur.apply(s,arguments)},onChange:n[2]||=function(){return s.onChange&&s.onChange.apply(s,arguments)}},s.getPTOptions(`input`)),null,16,ve),x(`div`,c({class:e.cx(`slider`)},s.getPTOptions(`slider`),{"data-p":s.dataP}),[x(`div`,c({class:e.cx(`handle`)},s.getPTOptions(`handle`),{"data-p":s.dataP}),[t(e.$slots,`handle`,{checked:s.checked})],16,be)],16,ye)],16,_e)}Q.render=$;var xe={key:0,class:`ack-topology`},Se=M(S({__name:`AnomalyAckDialog`,props:{target:{}},emits:[`close`,`done`],setup(t,{emit:r}){let a=t,o=r,s=new Set([`orphan_idle`,`replaced`]),c=e(``),d=e(!1),y=e(!1),S=e(``);n(()=>a.target,()=>{c.value=``,d.value=!1,S.value=``});let C=_(()=>!!a.target&&!!a.target.inn&&s.has(a.target.kind));async function w(){let e=a.target;if(e&&c.value.trim()){y.value=!0;try{d.value&&C.value?await v.putTopology({inn:e.inn,kpp:e.kpp,edoId:e.edoId,purpose:c.value.trim()}):await v.acknowledge(e.edoId,e.fingerprint,c.value),o(`done`)}catch(e){S.value=e instanceof Error?e.message:`Не удалось сохранить пометку.`}finally{y.value=!1}}}return(e,n)=>(i(),p(g(ne),{visible:t.target!==null,modal:``,header:`Скрыть находку как допустимую`,style:{width:`28rem`,maxWidth:`calc(100vw - 32px)`},"onUpdate:visible":n[3]||=e=>o(`close`)},{footer:u(()=>[f(g(O),{label:`Отмена`,text:``,onClick:n[2]||=e=>o(`close`)}),f(g(O),{label:`Пометить`,loading:y.value,disabled:!c.value.trim(),onClick:w},null,8,[`loading`,`disabled`])]),default:u(()=>[n[5]||=x(`p`,{class:`ack-hint`},` Пометка скроет именно это состояние. Если оно изменится — трафик пойдёт, сменится логин или владелец — находка появится снова. `,-1),f(g(z),{modelValue:c.value,"onUpdate:modelValue":n[0]||=e=>c.value=e,class:`ack-reason`,rows:`3`,"auto-resize":``,placeholder:`Причина, например: разные виды деятельности`},null,8,[`modelValue`]),C.value?(i(),b(`label`,xe,[f(g(Q),{modelValue:d.value,"onUpdate:modelValue":n[1]||=e=>d.value=e},null,8,[`modelValue`]),n[4]||=x(`span`,null,` Законная связь организации: запомнить в реестре топологии — гасит и будущие сигналы «без владельца» и «замена» по этой паре ИНН и идентификатора `,-1)])):m(``,!0),S.value?(i(),p(g(ae),{key:1,severity:`error`,closable:!1},{default:u(()=>[h(l(S.value),1)]),_:1})):m(``,!0)]),_:1},8,[`visible`]))}}),[[`__scopeId`,`data-v-a4b930cc`]]),Ce={class:`month-bars`},we=[`aria-label`],Te={class:`area`},Ee=[`aria-label`],De={key:2,class:`mark missing`},Oe={class:`tip`},ke={class:`axis`},Ae={class:`year`},je={class:`as-table`},Me=M(S({__name:`MonthBars`,props:{bars:{},limit:{},unit:{},title:{}},setup(e){let t=e,n=_(()=>Math.max(1,t.limit??0,...t.bars.map(e=>e.value??0))),r=_(()=>{let e=t.bars.filter(e=>e.value!==null),n=e.at(-1),r=e.reduce((e,t)=>e===void 0||(t.value??0)>(e.value??0)?t:e,void 0);return new Set([n?.key,r?.key])});function o(e){return`${(e??0)/n.value*100}%`}function c(e){let n=e.value===null?`нет данных`:`${e.value} ${t.unit}`;return[e.label,n,e.note].filter(Boolean).join(` · `)}function u(e){let t=e.lastIndexOf(` `);return t>0?e.slice(0,t):e}function d(e){let t=e.lastIndexOf(` `);return t>0?e.slice(t):``}return(t,n)=>(i(),b(`figure`,Ce,[x(`div`,{class:`plot`,role:`img`,"aria-label":e.title},[x(`div`,Te,[e.limit?(i(),b(`div`,{key:0,class:`limit`,style:C({bottom:o(e.limit)})},[x(`span`,null,`лимит `+l(e.limit),1)],4)):m(``,!0),(i(!0),b(y,null,a(e.bars,e=>(i(),b(`div`,{key:e.key,class:`month-col`,tabindex:`0`,"aria-label":c(e)},[e.value!==null&&r.value.has(e.key)?(i(),b(`span`,{key:0,class:`value`,style:C({bottom:o(e.value)})},l(e.value),5)):m(``,!0),e.value===null?(i(),b(`div`,De)):(i(),b(`div`,{key:1,class:s([`mark`,{over:e.over}]),style:C({height:o(e.value)})},null,6)),x(`span`,Oe,l(c(e)),1)],8,Ee))),128))])],8,we),x(`div`,ke,[(i(!0),b(y,null,a(e.bars,e=>(i(),b(`span`,{key:e.key},[h(l(u(e.label)),1),x(`span`,Ae,l(d(e.label)),1)]))),128))]),x(`details`,je,[n[0]||=x(`summary`,null,`Таблицей`,-1),x(`table`,null,[x(`tbody`,null,[(i(!0),b(y,null,a(e.bars,e=>(i(),b(`tr`,{key:e.key},[x(`th`,null,l(e.label),1),x(`td`,null,l(e.value??`—`),1),x(`td`,null,l(e.note),1)]))),128))])])])]))}}),[[`__scopeId`,`data-v-2a4cd37d`]]);export{V as a,X as i,Se as n,Z as r,Me as t};