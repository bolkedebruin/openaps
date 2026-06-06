var Z4=globalThis,o4=Z4.ShadowRoot&&(Z4.ShadyCSS===void 0||Z4.ShadyCSS.nativeShadow)&&"adoptedStyleSheets"in Document.prototype&&"replace"in CSSStyleSheet.prototype,r4=Symbol(),N3=new WeakMap;class p4{constructor(Q,Y,K){if(this._$cssResult$=!0,K!==r4)throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");this.cssText=Q,this._strings=Y}get styleSheet(){let Q=this._styleSheet,Y=this._strings;if(o4&&Q===void 0){let K=Y!==void 0&&Y.length===1;if(K)Q=N3.get(Y);if(Q===void 0){if((this._styleSheet=Q=new CSSStyleSheet).replaceSync(this.cssText),K)N3.set(Y,Q)}}return Q}toString(){return this.cssText}}var VQ=(Q)=>{if(Q._$cssResult$===!0)return Q.cssText;else if(typeof Q==="number")return Q;else throw Error(`Value passed to 'css' function must be a 'css' function result: ${Q}. Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.`)},NQ=(Q)=>new p4(typeof Q==="string"?Q:String(Q),void 0,r4),L=(Q,...Y)=>{let K=Q.length===1?Q[0]:Y.reduce((X,G,z)=>X+VQ(G)+Q[z+1],Q[0]);return new p4(K,Q,r4)},O3=(Q,Y)=>{if(o4)Q.adoptedStyleSheets=Y.map((K)=>K instanceof CSSStyleSheet?K:K.styleSheet);else for(let K of Y){let X=document.createElement("style"),G=Z4.litNonce;if(G!==void 0)X.setAttribute("nonce",G);X.textContent=K.cssText,Q.appendChild(X)}},OQ=(Q)=>{let Y="";for(let K of Q.cssRules)Y+=K.cssText;return NQ(Y)},l4=o4?(Q)=>Q:(Q)=>Q instanceof CSSStyleSheet?OQ(Q):Q;var{is:PQ,defineProperty:EQ,getOwnPropertyDescriptor:P3,getOwnPropertyNames:MQ,getOwnPropertySymbols:ZQ,getPrototypeOf:E3}=Object,RQ=!1,R=globalThis;if(RQ)R.customElements??=customElements;var _=!0,C,M3=R.trustedTypes,_Q=M3?M3.emptyScript:"",R3=_?R.reactiveElementPolyfillSupportDevMode:R.reactiveElementPolyfillSupport;if(_)R.litIssuedWarnings??=new Set,C=(Q,Y)=>{if(Y+=` See https://lit.dev/msg/${Q} for more information.`,!R.litIssuedWarnings.has(Y)&&!R.litIssuedWarnings.has(Q))console.warn(Y),R.litIssuedWarnings.add(Y)},queueMicrotask(()=>{if(C("dev-mode","Lit is in dev mode. Not recommended for production!"),R.ShadyDOM?.inUse&&R3===void 0)C("polyfill-support-missing","Shadow DOM is being polyfilled via `ShadyDOM` but the `polyfill-support` module has not been loaded.")});var wQ=_?(Q)=>{if(!R.emitLitDebugLogEvents)return;R.dispatchEvent(new CustomEvent("lit-debug",{detail:Q}))}:void 0,t=(Q,Y)=>Q,d4={toAttribute(Q,Y){switch(Y){case Boolean:Q=Q?_Q:null;break;case Object:case Array:Q=Q==null?Q:JSON.stringify(Q);break}return Q},fromAttribute(Q,Y){let K=Q;switch(Y){case Boolean:K=Q!==null;break;case Number:K=Q===null?null:Number(Q);break;case Object:case Array:try{K=JSON.parse(Q)}catch(X){K=null}break}return K}},_3=(Q,Y)=>!PQ(Q,Y),Z3={attribute:!0,type:String,converter:d4,reflect:!1,useDefault:!1,hasChanged:_3};Symbol.metadata??=Symbol("metadata");R.litPropertyMetadata??=new WeakMap;class w extends HTMLElement{static addInitializer(Q){this.__prepare(),(this._initializers??=[]).push(Q)}static get observedAttributes(){return this.finalize(),this.__attributeToPropertyMap&&[...this.__attributeToPropertyMap.keys()]}static createProperty(Q,Y=Z3){if(Y.state)Y.attribute=!1;if(this.__prepare(),this.prototype.hasOwnProperty(Q))Y=Object.create(Y),Y.wrapped=!0;if(this.elementProperties.set(Q,Y),!Y.noAccessor){let K=_?Symbol.for(`${String(Q)} (@property() cache)`):Symbol(),X=this.getPropertyDescriptor(Q,K,Y);if(X!==void 0)EQ(this.prototype,Q,X)}}static getPropertyDescriptor(Q,Y,K){let{get:X,set:G}=P3(this.prototype,Q)??{get(){return this[Y]},set(z){this[Y]=z}};if(_&&X==null){if("value"in(P3(this.prototype,Q)??{}))throw Error(`Field ${JSON.stringify(String(Q))} on ${this.name} was declared as a reactive property but it's actually declared as a value on the prototype. Usually this is due to using @property or @state on a method.`);C("reactive-property-without-getter",`Field ${JSON.stringify(String(Q))} on ${this.name} was declared as a reactive property but it does not have a getter. This will be an error in a future version of Lit.`)}return{get:X,set(z){let B=X?.call(this);G?.call(this,z),this.requestUpdate(Q,B,K)},configurable:!0,enumerable:!0}}static getPropertyOptions(Q){return this.elementProperties.get(Q)??Z3}static __prepare(){if(this.hasOwnProperty(t("elementProperties",this)))return;let Q=E3(this);if(Q.finalize(),Q._initializers!==void 0)this._initializers=[...Q._initializers];this.elementProperties=new Map(Q.elementProperties)}static finalize(){if(this.hasOwnProperty(t("finalized",this)))return;if(this.finalized=!0,this.__prepare(),this.hasOwnProperty(t("properties",this))){let Y=this.properties,K=[...MQ(Y),...ZQ(Y)];for(let X of K)this.createProperty(X,Y[X])}let Q=this[Symbol.metadata];if(Q!==null){let Y=litPropertyMetadata.get(Q);if(Y!==void 0)for(let[K,X]of Y)this.elementProperties.set(K,X)}this.__attributeToPropertyMap=new Map;for(let[Y,K]of this.elementProperties){let X=this.__attributeNameForProperty(Y,K);if(X!==void 0)this.__attributeToPropertyMap.set(X,Y)}if(this.elementStyles=this.finalizeStyles(this.styles),_){if(this.hasOwnProperty("createProperty"))C("no-override-create-property","Overriding ReactiveElement.createProperty() is deprecated. The override will not be called with standard decorators");if(this.hasOwnProperty("getPropertyDescriptor"))C("no-override-get-property-descriptor","Overriding ReactiveElement.getPropertyDescriptor() is deprecated. The override will not be called with standard decorators")}}static finalizeStyles(Q){let Y=[];if(Array.isArray(Q)){let K=new Set(Q.flat(1/0).reverse());for(let X of K)Y.unshift(l4(X))}else if(Q!==void 0)Y.push(l4(Q));return Y}static __attributeNameForProperty(Q,Y){let K=Y.attribute;return K===!1?void 0:typeof K==="string"?K:typeof Q==="string"?Q.toLowerCase():void 0}constructor(){super();this.__instanceProperties=void 0,this.isUpdatePending=!1,this.hasUpdated=!1,this.__reflectingProperty=null,this.__initialize()}__initialize(){this.__updatePromise=new Promise((Q)=>this.enableUpdating=Q),this._$changedProperties=new Map,this.__saveInstanceProperties(),this.requestUpdate(),this.constructor._initializers?.forEach((Q)=>Q(this))}addController(Q){if((this.__controllers??=new Set).add(Q),this.renderRoot!==void 0&&this.isConnected)Q.hostConnected?.()}removeController(Q){this.__controllers?.delete(Q)}__saveInstanceProperties(){let Q=new Map,Y=this.constructor.elementProperties;for(let K of Y.keys())if(this.hasOwnProperty(K))Q.set(K,this[K]),delete this[K];if(Q.size>0)this.__instanceProperties=Q}createRenderRoot(){let Q=this.shadowRoot??this.attachShadow(this.constructor.shadowRootOptions);return O3(Q,this.constructor.elementStyles),Q}connectedCallback(){this.renderRoot??=this.createRenderRoot(),this.enableUpdating(!0),this.__controllers?.forEach((Q)=>Q.hostConnected?.())}enableUpdating(Q){}disconnectedCallback(){this.__controllers?.forEach((Q)=>Q.hostDisconnected?.())}attributeChangedCallback(Q,Y,K){this._$attributeToProperty(Q,K)}__propertyToAttribute(Q,Y){let X=this.constructor.elementProperties.get(Q),G=this.constructor.__attributeNameForProperty(Q,X);if(G!==void 0&&X.reflect===!0){let B=(X.converter?.toAttribute!==void 0?X.converter:d4).toAttribute(Y,X.type);if(_&&this.constructor.enabledWarnings.includes("migration")&&B===void 0)C("undefined-attribute-value",`The attribute value for the ${Q} property is undefined on element ${this.localName}. The attribute will be removed, but in the previous version of \`ReactiveElement\`, the attribute would not have changed.`);if(this.__reflectingProperty=Q,B==null)this.removeAttribute(G);else this.setAttribute(G,B);this.__reflectingProperty=null}}_$attributeToProperty(Q,Y){let K=this.constructor,X=K.__attributeToPropertyMap.get(Q);if(X!==void 0&&this.__reflectingProperty!==X){let G=K.getPropertyOptions(X),z=typeof G.converter==="function"?{fromAttribute:G.converter}:G.converter?.fromAttribute!==void 0?G.converter:d4;this.__reflectingProperty=X;let B=z.fromAttribute(Y,G.type);this[X]=B??this.__defaultValues?.get(X)??B,this.__reflectingProperty=null}}requestUpdate(Q,Y,K,X=!1,G){if(Q!==void 0){if(_&&Q instanceof Event)C("","The requestUpdate() method was called with an Event as the property name. This is probably a mistake caused by binding this.requestUpdate as an event listener. Instead bind a function that will call it with no arguments: () => this.requestUpdate()");let z=this.constructor;if(X===!1)G=this[Q];if(K??=z.getPropertyOptions(Q),(K.hasChanged??_3)(G,Y)||K.useDefault&&K.reflect&&G===this.__defaultValues?.get(Q)&&!this.hasAttribute(z.__attributeNameForProperty(Q,K)))this._$changeProperty(Q,Y,K);else return}if(this.isUpdatePending===!1)this.__updatePromise=this.__enqueueUpdate()}_$changeProperty(Q,Y,{useDefault:K,reflect:X,wrapped:G},z){if(K&&!(this.__defaultValues??=new Map).has(Q)){if(this.__defaultValues.set(Q,z??Y??this[Q]),G!==!0||z!==void 0)return}if(!this._$changedProperties.has(Q)){if(!this.hasUpdated&&!K)Y=void 0;this._$changedProperties.set(Q,Y)}if(X===!0&&this.__reflectingProperty!==Q)(this.__reflectingProperties??=new Set).add(Q)}async __enqueueUpdate(){this.isUpdatePending=!0;try{await this.__updatePromise}catch(Y){Promise.reject(Y)}let Q=this.scheduleUpdate();if(Q!=null)await Q;return!this.isUpdatePending}scheduleUpdate(){let Q=this.performUpdate();if(_&&this.constructor.enabledWarnings.includes("async-perform-update")&&typeof Q?.then==="function")C("async-perform-update",`Element ${this.localName} returned a Promise from performUpdate(). This behavior is deprecated and will be removed in a future version of ReactiveElement.`);return Q}performUpdate(){if(!this.isUpdatePending)return;if(wQ?.({kind:"update"}),!this.hasUpdated){if(this.renderRoot??=this.createRenderRoot(),_){let G=[...this.constructor.elementProperties.keys()].filter((z)=>this.hasOwnProperty(z)&&(z in E3(this)));if(G.length)throw Error(`The following properties on element ${this.localName} will not trigger updates as expected because they are set using class fields: ${G.join(", ")}. Native class fields and some compiled output will overwrite accessors used for detecting changes. See https://lit.dev/msg/class-field-shadowing for more information.`)}if(this.__instanceProperties){for(let[X,G]of this.__instanceProperties)this[X]=G;this.__instanceProperties=void 0}let K=this.constructor.elementProperties;if(K.size>0)for(let[X,G]of K){let{wrapped:z}=G,B=this[X];if(z===!0&&!this._$changedProperties.has(X)&&B!==void 0)this._$changeProperty(X,void 0,G,B)}}let Q=!1,Y=this._$changedProperties;try{if(Q=this.shouldUpdate(Y),Q)this.willUpdate(Y),this.__controllers?.forEach((K)=>K.hostUpdate?.()),this.update(Y);else this.__markUpdated()}catch(K){throw Q=!1,this.__markUpdated(),K}if(Q)this._$didUpdate(Y)}willUpdate(Q){}_$didUpdate(Q){if(this.__controllers?.forEach((Y)=>Y.hostUpdated?.()),!this.hasUpdated)this.hasUpdated=!0,this.firstUpdated(Q);if(this.updated(Q),_&&this.isUpdatePending&&this.constructor.enabledWarnings.includes("change-in-update"))C("change-in-update",`Element ${this.localName} scheduled an update (generally because a property was set) after an update completed, causing a new update to be scheduled. This is inefficient and should be avoided unless the next update can only be scheduled as a side effect of the previous update.`)}__markUpdated(){this._$changedProperties=new Map,this.isUpdatePending=!1}get updateComplete(){return this.getUpdateComplete()}getUpdateComplete(){return this.__updatePromise}shouldUpdate(Q){return!0}update(Q){this.__reflectingProperties&&=this.__reflectingProperties.forEach((Y)=>this.__propertyToAttribute(Y,this[Y])),this.__markUpdated()}updated(Q){}firstUpdated(Q){}}w.elementStyles=[];w.shadowRootOptions={mode:"open"};w[t("elementProperties",w)]=new Map;w[t("finalized",w)]=new Map;R3?.({ReactiveElement:w});if(_){w.enabledWarnings=["change-in-update","async-perform-update"];let Q=function(Y){if(!Y.hasOwnProperty(t("enabledWarnings",Y)))Y.enabledWarnings=Y.enabledWarnings.slice()};w.enableWarning=function(Y){if(Q(this),!this.enabledWarnings.includes(Y))this.enabledWarnings.push(Y)},w.disableWarning=function(Y){Q(this);let K=this.enabledWarnings.indexOf(Y);if(K>=0)this.enabledWarnings.splice(K,1)}}(R.reactiveElementVersions??=[]).push("2.1.2");if(_&&R.reactiveElementVersions.length>1)queueMicrotask(()=>{C("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var T=globalThis,V=(Q)=>{if(!T.emitLitDebugLogEvents)return;T.dispatchEvent(new CustomEvent("lit-debug",{detail:Q}))},TQ=0,U4;T.litIssuedWarnings??=new Set,U4=(Q,Y)=>{if(Y+=Q?` See https://lit.dev/msg/${Q} for more information.`:"",!T.litIssuedWarnings.has(Y)&&!T.litIssuedWarnings.has(Q))console.warn(Y),T.litIssuedWarnings.add(Y)},queueMicrotask(()=>{U4("dev-mode","Lit is in dev mode. Not recommended for production!")});var k=T.ShadyDOM?.inUse&&T.ShadyDOM?.noPatch===!0?T.ShadyDOM.wrap:(Q)=>Q,R4=T.trustedTypes,w3=R4?R4.createPolicy("lit-html",{createHTML:(Q)=>Q}):void 0,SQ=(Q)=>Q,S4=(Q,Y,K)=>SQ,bQ=(Q)=>{if(n!==S4)throw Error("Attempted to overwrite existing lit-html security policy. setSanitizeDOMValueFactory should be called at most once.");n=Q},CQ=()=>{n=S4},a4=(Q,Y,K)=>{return n(Q,Y,K)},g3="$lit$",y=`lit$${Math.random().toFixed(9).slice(2)}$`,v3="?"+y,kQ=`<${v3}>`,l=document,j4=()=>l.createComment(""),$4=(Q)=>Q===null||typeof Q!="object"&&typeof Q!="function",e4=Array.isArray,xQ=(Q)=>e4(Q)||typeof Q?.[Symbol.iterator]==="function",n4=`[ 	
\f\r]`,gQ=`[^ 	
\f\r"'\`<>=]`,vQ=`[^\\s"'>=/]`,H4=/<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g,T3=1,i4=2,hQ=3,S3=/-->/g,b3=/>/g,r=new RegExp(`>|${n4}(?:(${vQ}+)(${n4}*=${n4}*(?:${gQ}|("|')|))|$)`,"g"),cQ=0,C3=1,yQ=2,k3=3,s4=/'/g,t4=/"/g,h3=/^(?:script|style|textarea|title)$/i,mQ=1,_4=2,w4=3,Q3=1,T4=2,uQ=3,fQ=4,oQ=5,Y3=6,rQ=7,K3=(Q)=>(Y,...K)=>{if(Y.some((X)=>X===void 0))console.warn(`Some template strings are undefined.
This is probably caused by illegal octal escape sequences.`);if(K.some((X)=>X?._$litStatic$))U4("",`Static values 'literal' or 'unsafeStatic' cannot be used as values to non-static templates.
Please use the static 'html' tag function. See https://lit.dev/docs/templates/expressions/#static-expressions`);return{["_$litType$"]:Q,strings:Y,values:K}},q=K3(mQ),S=K3(_4),v8=K3(w4),d=Symbol.for("lit-noChange"),J=Symbol.for("lit-nothing"),x3=new WeakMap,p=l.createTreeWalker(l,129),n=S4;function c3(Q,Y){if(!e4(Q)||!Q.hasOwnProperty("raw")){let K="invalid template strings array";throw K=`
          Internal Error: expected template strings to be an array
          with a 'raw' field. Faking a template strings array by
          calling html or svg like an ordinary function is effectively
          the same as calling unsafeHtml and can lead to major security
          issues, e.g. opening your code up to XSS attacks.
          If you're using the html or svg tagged template functions normally
          and still seeing this error, please file a bug at
          https://github.com/lit/lit/issues/new?template=bug_report.md
          and include information about your build tooling, if any.
        `.trim().replace(/\n */g,`
`),Error(K)}return w3!==void 0?w3.createHTML(Y):Y}var pQ=(Q,Y)=>{let K=Q.length-1,X=[],G=Y===_4?"<svg>":Y===w4?"<math>":"",z,B=H4;for(let W=0;W<K;W++){let U=Q[W],j=-1,A,F=0,I;while(F<U.length){if(B.lastIndex=F,I=B.exec(U),I===null)break;if(F=B.lastIndex,B===H4){if(I[T3]==="!--")B=S3;else if(I[T3]!==void 0)B=b3;else if(I[i4]!==void 0){if(h3.test(I[i4]))z=new RegExp(`</${I[i4]}`,"g");B=r}else if(I[hQ]!==void 0)throw Error("Bindings in tag names are not supported. Please use static templates instead. See https://lit.dev/docs/templates/expressions/#static-expressions")}else if(B===r)if(I[cQ]===">")B=z??H4,j=-1;else if(I[C3]===void 0)j=-2;else j=B.lastIndex-I[yQ].length,A=I[C3],B=I[k3]===void 0?r:I[k3]==='"'?t4:s4;else if(B===t4||B===s4)B=r;else if(B===S3||B===b3)B=H4;else B=r,z=void 0}console.assert(j===-1||B===r||B===s4||B===t4,"unexpected parse state B");let P=B===r&&Q[W+1].startsWith("/>")?" ":"";G+=B===H4?U+kQ:j>=0?(X.push(A),U.slice(0,j)+g3+U.slice(j))+y+P:U+y+(j===-2?W:P)}let H=G+(Q[K]||"<?>")+(Y===_4?"</svg>":Y===w4?"</math>":"");return[c3(Q,H),X]};class A4{constructor({strings:Q,["_$litType$"]:Y},K){this.parts=[];let X,G=0,z=0,B=Q.length-1,H=this.parts,[W,U]=pQ(Q,Y);if(this.el=A4.createElement(W,K),p.currentNode=this.el.content,Y===_4||Y===w4){let j=this.el.content.firstChild;j.replaceWith(...j.childNodes)}while((X=p.nextNode())!==null&&H.length<B){if(X.nodeType===1){{let j=X.localName;if(/^(?:textarea|template)$/i.test(j)&&X.innerHTML.includes(y)){let A=`Expressions are not supported inside \`${j}\` elements. See https://lit.dev/msg/expression-in-${j} for more information.`;if(j==="template")throw Error(A);else U4("",A)}}if(X.hasAttributes()){for(let j of X.getAttributeNames())if(j.endsWith(g3)){let A=U[z++],I=X.getAttribute(j).split(y),P=/([.?@])?(.*)/.exec(A);H.push({type:Q3,index:G,name:P[2],strings:I,ctor:P[1]==="."?m3:P[1]==="?"?u3:P[1]==="@"?f3:L4}),X.removeAttribute(j)}else if(j.startsWith(y))H.push({type:Y3,index:G}),X.removeAttribute(j)}if(h3.test(X.tagName)){let j=X.textContent.split(y),A=j.length-1;if(A>0){X.textContent=R4?R4.emptyScript:"";for(let F=0;F<A;F++)X.append(j[F],j4()),p.nextNode(),H.push({type:T4,index:++G});X.append(j[A],j4())}}}else if(X.nodeType===8)if(X.data===v3)H.push({type:T4,index:G});else{let A=-1;while((A=X.data.indexOf(y,A+1))!==-1)H.push({type:rQ,index:G}),A+=y.length-1}G++}if(U.length!==z)throw Error('Detected duplicate attribute bindings. This occurs if your template has duplicate attributes on an element tag. For example "<input ?disabled=${true} ?disabled=${false}>" contains a duplicate "disabled" attribute. The error was detected in the following template: \n`'+Q.join("${...}")+"`");V&&V({kind:"template prep",template:this,clonableTemplate:this.el,parts:this.parts,strings:Q})}static createElement(Q,Y){let K=l.createElement("template");return K.innerHTML=Q,K}}function a(Q,Y,K=Q,X){if(Y===d)return Y;let G=X!==void 0?K.__directives?.[X]:K.__directive,z=$4(Y)?void 0:Y._$litDirective$;if(G?.constructor!==z){if(G?._$notifyDirectiveConnectionChanged?.(!1),z===void 0)G=void 0;else G=new z(Q),G._$initialize(Q,K,X);if(X!==void 0)(K.__directives??=[])[X]=G;else K.__directive=G}if(G!==void 0)Y=a(Q,G._$resolve(Q,Y.values),G,X);return Y}class y3{constructor(Q,Y){this._$parts=[],this._$disconnectableChildren=void 0,this._$template=Q,this._$parent=Y}get parentNode(){return this._$parent.parentNode}get _$isConnected(){return this._$parent._$isConnected}_clone(Q){let{el:{content:Y},parts:K}=this._$template,X=(Q?.creationScope??l).importNode(Y,!0);p.currentNode=X;let G=p.nextNode(),z=0,B=0,H=K[0];while(H!==void 0){if(z===H.index){let W;if(H.type===T4)W=new D4(G,G.nextSibling,this,Q);else if(H.type===Q3)W=new H.ctor(G,H.name,H.strings,this,Q);else if(H.type===Y3)W=new o3(G,this,Q);this._$parts.push(W),H=K[++B]}if(z!==H?.index)G=p.nextNode(),z++}return p.currentNode=l,X}_update(Q){let Y=0;for(let K of this._$parts){if(K!==void 0)if(V&&V({kind:"set part",part:K,value:Q[Y],valueIndex:Y,values:Q,templateInstance:this}),K.strings!==void 0)K._$setValue(Q,K,Y),Y+=K.strings.length-2;else K._$setValue(Q[Y]);Y++}}}class D4{get _$isConnected(){return this._$parent?._$isConnected??this.__isConnected}constructor(Q,Y,K,X){this.type=T4,this._$committedValue=J,this._$disconnectableChildren=void 0,this._$startNode=Q,this._$endNode=Y,this._$parent=K,this.options=X,this.__isConnected=X?.isConnected??!0,this._textSanitizer=void 0}get parentNode(){let Q=k(this._$startNode).parentNode,Y=this._$parent;if(Y!==void 0&&Q?.nodeType===11)Q=Y.parentNode;return Q}get startNode(){return this._$startNode}get endNode(){return this._$endNode}_$setValue(Q,Y=this){if(this.parentNode===null)throw Error("This `ChildPart` has no `parentNode` and therefore cannot accept a value. This likely means the element containing the part was manipulated in an unsupported way outside of Lit's control such that the part's marker nodes were ejected from DOM. For example, setting the element's `innerHTML` or `textContent` can do this.");if(Q=a(this,Q,Y),$4(Q)){if(Q===J||Q==null||Q===""){if(this._$committedValue!==J)V&&V({kind:"commit nothing to child",start:this._$startNode,end:this._$endNode,parent:this._$parent,options:this.options}),this._$clear();this._$committedValue=J}else if(Q!==this._$committedValue&&Q!==d)this._commitText(Q)}else if(Q._$litType$!==void 0)this._commitTemplateResult(Q);else if(Q.nodeType!==void 0){if(this.options?.host===Q){this._commitText("[probable mistake: rendered a template's host in itself (commonly caused by writing ${this} in a template]"),console.warn("Attempted to render the template host",Q,"inside itself. This is almost always a mistake, and in dev mode ","we render some warning text. In production however, we'll ","render it, which will usually result in an error, and sometimes ","in the element disappearing from the DOM.");return}this._commitNode(Q)}else if(xQ(Q))this._commitIterable(Q);else this._commitText(Q)}_insert(Q){return k(k(this._$startNode).parentNode).insertBefore(Q,this._$endNode)}_commitNode(Q){if(this._$committedValue!==Q){if(this._$clear(),n!==S4){let Y=this._$startNode.parentNode?.nodeName;if(Y==="STYLE"||Y==="SCRIPT"){let K="Forbidden";if(Y==="STYLE")K="Lit does not support binding inside style nodes. This is a security risk, as style injection attacks can exfiltrate data and spoof UIs. Consider instead using css`...` literals to compose styles, and do dynamic styling with css custom properties, ::parts, <slot>s, and by mutating the DOM rather than stylesheets.";else K="Lit does not support binding inside script nodes. This is a security risk, as it could allow arbitrary code execution.";throw Error(K)}}V&&V({kind:"commit node",start:this._$startNode,parent:this._$parent,value:Q,options:this.options}),this._$committedValue=this._insert(Q)}}_commitText(Q){if(this._$committedValue!==J&&$4(this._$committedValue)){let Y=k(this._$startNode).nextSibling;if(this._textSanitizer===void 0)this._textSanitizer=a4(Y,"data","property");Q=this._textSanitizer(Q),V&&V({kind:"commit text",node:Y,value:Q,options:this.options}),Y.data=Q}else{let Y=l.createTextNode("");if(this._commitNode(Y),this._textSanitizer===void 0)this._textSanitizer=a4(Y,"data","property");Q=this._textSanitizer(Q),V&&V({kind:"commit text",node:Y,value:Q,options:this.options}),Y.data=Q}this._$committedValue=Q}_commitTemplateResult(Q){let{values:Y,["_$litType$"]:K}=Q,X=typeof K==="number"?this._$getTemplate(Q):(K.el===void 0&&(K.el=A4.createElement(c3(K.h,K.h[0]),this.options)),K);if(this._$committedValue?._$template===X)V&&V({kind:"template updating",template:X,instance:this._$committedValue,parts:this._$committedValue._$parts,options:this.options,values:Y}),this._$committedValue._update(Y);else{let G=new y3(X,this),z=G._clone(this.options);V&&V({kind:"template instantiated",template:X,instance:G,parts:G._$parts,options:this.options,fragment:z,values:Y}),G._update(Y),V&&V({kind:"template instantiated and updated",template:X,instance:G,parts:G._$parts,options:this.options,fragment:z,values:Y}),this._commitNode(z),this._$committedValue=G}}_$getTemplate(Q){let Y=x3.get(Q.strings);if(Y===void 0)x3.set(Q.strings,Y=new A4(Q));return Y}_commitIterable(Q){if(!e4(this._$committedValue))this._$committedValue=[],this._$clear();let Y=this._$committedValue,K=0,X;for(let G of Q){if(K===Y.length)Y.push(X=new D4(this._insert(j4()),this._insert(j4()),this,this.options));else X=Y[K];X._$setValue(G),K++}if(K<Y.length)this._$clear(X&&k(X._$endNode).nextSibling,K),Y.length=K}_$clear(Q=k(this._$startNode).nextSibling,Y){this._$notifyConnectionChanged?.(!1,!0,Y);while(Q!==this._$endNode){let K=k(Q).nextSibling;k(Q).remove(),Q=K}}setConnected(Q){if(this._$parent===void 0)this.__isConnected=Q,this._$notifyConnectionChanged?.(Q);else throw Error("part.setConnected() may only be called on a RootPart returned from render().")}}class L4{get tagName(){return this.element.tagName}get _$isConnected(){return this._$parent._$isConnected}constructor(Q,Y,K,X,G){if(this.type=Q3,this._$committedValue=J,this._$disconnectableChildren=void 0,this.element=Q,this.name=Y,this._$parent=X,this.options=G,K.length>2||K[0]!==""||K[1]!=="")this._$committedValue=Array(K.length-1).fill(new String),this.strings=K;else this._$committedValue=J;this._sanitizer=void 0}_$setValue(Q,Y=this,K,X){let G=this.strings,z=!1;if(G===void 0){if(Q=a(this,Q,Y,0),z=!$4(Q)||Q!==this._$committedValue&&Q!==d,z)this._$committedValue=Q}else{let B=Q;Q=G[0];let H,W;for(H=0;H<G.length-1;H++){if(W=a(this,B[K+H],Y,H),W===d)W=this._$committedValue[H];if(z||=!$4(W)||W!==this._$committedValue[H],W===J)Q=J;else if(Q!==J)Q+=(W??"")+G[H+1];this._$committedValue[H]=W}}if(z&&!X)this._commitValue(Q)}_commitValue(Q){if(Q===J)k(this.element).removeAttribute(this.name);else{if(this._sanitizer===void 0)this._sanitizer=n(this.element,this.name,"attribute");Q=this._sanitizer(Q??""),V&&V({kind:"commit attribute",element:this.element,name:this.name,value:Q,options:this.options}),k(this.element).setAttribute(this.name,Q??"")}}}class m3 extends L4{constructor(){super(...arguments);this.type=uQ}_commitValue(Q){if(this._sanitizer===void 0)this._sanitizer=n(this.element,this.name,"property");Q=this._sanitizer(Q),V&&V({kind:"commit property",element:this.element,name:this.name,value:Q,options:this.options}),this.element[this.name]=Q===J?void 0:Q}}class u3 extends L4{constructor(){super(...arguments);this.type=fQ}_commitValue(Q){V&&V({kind:"commit boolean attribute",element:this.element,name:this.name,value:!!(Q&&Q!==J),options:this.options}),k(this.element).toggleAttribute(this.name,!!Q&&Q!==J)}}class f3 extends L4{constructor(Q,Y,K,X,G){super(Q,Y,K,X,G);if(this.type=oQ,this.strings!==void 0)throw Error(`A \`<${Q.localName}>\` has a \`@${Y}=...\` listener with invalid content. Event listeners in templates must have exactly one expression and no surrounding text.`)}_$setValue(Q,Y=this){if(Q=a(this,Q,Y,0)??J,Q===d)return;let K=this._$committedValue,X=Q===J&&K!==J||Q.capture!==K.capture||Q.once!==K.once||Q.passive!==K.passive,G=Q!==J&&(K===J||X);if(V&&V({kind:"commit event listener",element:this.element,name:this.name,value:Q,options:this.options,removeListener:X,addListener:G,oldListener:K}),X)this.element.removeEventListener(this.name,this,K);if(G)this.element.addEventListener(this.name,this,Q);this._$committedValue=Q}handleEvent(Q){if(typeof this._$committedValue==="function")this._$committedValue.call(this.options?.host??this.element,Q);else this._$committedValue.handleEvent(Q)}}class o3{constructor(Q,Y,K){this.element=Q,this.type=Y3,this._$disconnectableChildren=void 0,this._$parent=Y,this.options=K}get _$isConnected(){return this._$parent._$isConnected}_$setValue(Q){V&&V({kind:"commit to element binding",element:this.element,value:Q,options:this.options}),a(this,Q)}}var lQ=T.litHtmlPolyfillSupportDevMode;lQ?.(A4,D4);(T.litHtmlVersions??=[]).push("3.3.3");if(T.litHtmlVersions.length>1)queueMicrotask(()=>{U4("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var W4=(Q,Y,K)=>{if(Y==null)throw TypeError(`The container to render into may not be ${Y}`);let X=TQ++,G=K?.renderBefore??Y,z=G._$litPart$;if(V&&V({kind:"begin render",id:X,value:Q,container:Y,options:K,part:z}),z===void 0){let B=K?.renderBefore??null;G._$litPart$=z=new D4(Y.insertBefore(j4(),B),B,void 0,K??{})}return z._$setValue(Q),V&&V({kind:"end render",id:X,value:Q,container:Y,options:K,part:z}),z};W4.setSanitizer=bQ,W4.createSanitizer=a4,W4._testOnlyClearSanitizerFactoryDoNotCallOrElse=CQ;var dQ=(Q,Y)=>Q,X3=!0,m=globalThis,r3;if(X3)m.litIssuedWarnings??=new Set,r3=(Q,Y)=>{if(Y+=` See https://lit.dev/msg/${Q} for more information.`,!m.litIssuedWarnings.has(Y)&&!m.litIssuedWarnings.has(Q))console.warn(Y),m.litIssuedWarnings.add(Y)};class $ extends w{constructor(){super(...arguments);this.renderOptions={host:this},this.__childPart=void 0}createRenderRoot(){let Q=super.createRenderRoot();return this.renderOptions.renderBefore??=Q.firstChild,Q}update(Q){let Y=this.render();if(!this.hasUpdated)this.renderOptions.isConnected=this.isConnected;super.update(Q),this.__childPart=W4(Y,this.renderRoot,this.renderOptions)}connectedCallback(){super.connectedCallback(),this.__childPart?.setConnected(!0)}disconnectedCallback(){super.disconnectedCallback(),this.__childPart?.setConnected(!1)}render(){return d}}$._$litElement$=!0;$[dQ("finalized",$)]=!0;m.litElementHydrateSupport?.({LitElement:$});var nQ=X3?m.litElementPolyfillSupportDevMode:m.litElementPolyfillSupport;nQ?.({LitElement:$});(m.litElementVersions??=[]).push("4.2.2");if(X3&&m.litElementVersions.length>1)queueMicrotask(()=>{r3("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});async function F4(Q,Y){let K=(await Q.text()).trim();if(K){try{let X=JSON.parse(K);if(typeof X?.error==="string"&&X.error)return X.error}catch{}return K}return`${Y}: ${Q.status}`}async function v(Q){let Y=await fetch(Q,{credentials:"same-origin"});if(!Y.ok)throw Error(await F4(Y,Q));return await Y.json()}async function G3(Q,Y){let K=await fetch(Q,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Y)});if(!K.ok)throw Error(await F4(K,Q))}async function b(Q,Y){let K=await fetch(Q,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Y)});if(!K.ok)throw Error(await F4(K,Q));return await K.json()}async function p3(Q,Y){let K=await fetch(Q,{method:"PUT",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Y)});if(!K.ok)throw Error(await F4(K,Q));return await K.json()}function e(Q){if(!Q||!Q.op)return!1;return Q.stage!==""&&Q.stage!=="done"&&Q.stage!=="aborted"&&Q.stage!=="error"}async function l3(Q,Y){let K=await fetch(Q,{method:"DELETE",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Y)});if(!K.ok)throw Error(await F4(K,Q));return await K.json()}var D={authStatus:()=>v("/api/auth/status"),setup:(Q)=>b("/api/auth/setup",{password:Q}),login:(Q)=>G3("/api/auth/login",{password:Q}),logout:()=>G3("/api/auth/logout",{}),recover:(Q,Y)=>b("/api/auth/recover",{recovery_code:Q,password:Y}),changePassword:(Q,Y)=>G3("/api/auth/change-password",{current_password:Q,new_password:Y}),regenerateRecovery:()=>b("/api/auth/recovery",{}),fleet:()=>v("/api/fleet"),system:()=>v("/api/system"),history:()=>v("/api/history"),events:(Q={})=>{let Y=new URLSearchParams;if(Q.since_ms)Y.set("since_ms",String(Q.since_ms));if(Q.kind)Y.set("kind",Q.kind);if(Q.severity)Y.set("severity",Q.severity);if(Q.inverter_uid)Y.set("inverter_uid",Q.inverter_uid);if(Q.limit)Y.set("limit",String(Q.limit));let K=Y.toString();return v("/api/events"+(K?`?${K}`:""))},getSettings:async()=>{let Q=await v("/api/settings");if(Q.error)return{error:Q.error};return{settings:{ecu_id:Q.ecu_id,mac:Q.mac,pan_override:Q.pan_override,zigbee_type:Q.zigbee_type,channel:Q.channel,inverter_names:Q.inverter_names??{}}}},saveSettings:(Q)=>p3("/api/settings",Q),verifyPassword:async(Q)=>{let Y=await fetch("/api/auth/verify",{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify({password:Q})});if(Y.status===200)return!0;if(Y.status===401)return!1;let K=await Y.text();throw Error(K.trim()||`/api/auth/verify: ${Y.status}`)},setPower:(Q)=>b("/api/power",Q),profiles:()=>v("/api/profiles"),overlays:()=>v("/api/overlays"),selectBase:(Q)=>b("/api/profiles/base",{id:Q}),saveOverlay:(Q)=>p3("/api/profiles/overlay",Q),deleteOverlay:(Q,Y)=>l3("/api/profiles/overlay",{id:Q,uids:Y}),pairingScan:(Q={})=>b("/api/pairing/scan",Q),pairingAdd:(Q)=>b("/api/pairing/add",{serial:Q}),pairingReplace:(Q,Y)=>b("/api/pairing/replace",{old_uid:Q,new_serial:Y}),pairingRekey:(Q,Y=0)=>b("/api/pairing/rekey",{new_pan:Q,channel:Y}),pairingChangeChannel:(Q)=>b("/api/pairing/change-channel",{channel:Q}),pairingAbort:()=>b("/api/pairing/abort",{}),pairingStatus:()=>v("/api/pairing/status"),sshKeys:()=>v("/api/access/ssh-keys"),addSshKey:(Q,Y)=>b("/api/access/ssh-keys",{pubkey:Q,comment:Y}),removeSshKey:(Q)=>l3("/api/access/ssh-keys",{fingerprint:Q})};function d3(Q,Y){let K=new EventSource("/api/stream");return K.addEventListener("fleet",(X)=>{try{Q(JSON.parse(X.data))}catch{}}),K.onerror=()=>Y?.(),()=>K.close()}class n3 extends ${static properties={configured:{type:Boolean},error:{state:!0},busy:{state:!0},recoverMode:{state:!0},savedCode:{state:!0},copied:{state:!0}};constructor(){super();this.configured=!0,this.error="",this.busy=!1,this.recoverMode=!1,this.savedCode="",this.copied=!1}static styles=L`
    :host {
      display: grid;
      place-items: center;
      min-height: 100vh;
    }
    .box {
      width: 340px;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 28px;
    }
    h1 { font-size: 20px; margin: 0 0 4px; color: var(--text); }
    p { color: var(--muted); font-size: 13px; margin: 0 0 18px; }
    label { display: block; font-size: 12px; color: var(--muted); margin: 14px 0 6px; }
    label:first-of-type { margin-top: 0; }
    input {
      width: 100%;
      box-sizing: border-box;
      padding: 10px 12px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font-size: 14px;
    }
    button.primary {
      width: 100%;
      margin-top: 18px;
      padding: 10px;
      background: var(--accent);
      color: #04222b;
      border: none;
      border-radius: 8px;
      font-weight: 700;
      cursor: pointer;
    }
    button.primary:disabled { opacity: 0.6; cursor: default; }
    .err { color: var(--err); font-size: 13px; margin-top: 12px; min-height: 16px; }
    .brand { color: var(--accent); font-weight: 700; letter-spacing: 0.04em; }
    .link {
      display: inline-block;
      margin-top: 16px;
      background: none;
      border: none;
      padding: 0;
      color: var(--muted);
      font-size: 13px;
      text-decoration: underline;
      cursor: pointer;
    }
    .link:hover { color: var(--text); }
    .code {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 18px;
      letter-spacing: 0.06em;
      text-align: center;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 14px;
      color: var(--text);
      user-select: all;
      word-break: break-all;
    }
    .warn { color: var(--text); font-size: 13px; margin: 0 0 14px; }
    .copy {
      width: 100%;
      margin-top: 10px;
      padding: 8px;
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 8px;
      font-size: 13px;
      cursor: pointer;
    }
    .copy:hover { color: var(--text); border-color: var(--muted); }
  `;firstUpdated(){this.focusFirst()}updated(Q){if(Q.has("recoverMode")||Q.has("savedCode"))this.focusFirst()}focusFirst(){this.renderRoot.querySelector("input")?.focus()}val(Q){return this.renderRoot.querySelector(`#${Q}`)?.value??""}async submit(Q){if(Q.preventDefault(),this.busy)return;this.error="";let Y=!this.configured,K=this.configured&&this.recoverMode;if(Y||K){if(this.val("pw")!==this.val("pw2")){this.error="Passwords do not match.";return}}this.busy=!0;try{if(Y){let X=await D.setup(this.val("pw"));this.savedCode=X.recovery_code}else if(K){let X=await D.recover(this.val("code"),this.val("pw"));this.savedCode=X.recovery_code}else await D.login(this.val("pw")),this.done()}catch(X){this.error=X.message||"failed"}finally{this.busy=!1}}done(){this.dispatchEvent(new CustomEvent("authed",{bubbles:!0,composed:!0}))}async copyCode(){try{await navigator.clipboard?.writeText(this.savedCode),this.copied=!0}catch{}}render(){if(this.savedCode)return this.renderSaved();let Q=!this.configured,Y=this.configured&&this.recoverMode,K=Y?"Reset password":"ECU Console",X=Q?"First run — choose an operator password (min 8 characters).":Y?"Enter your recovery code and a new password.":"Enter the operator password.";return q`
      <form class="box" @submit=${this.submit}>
        <h1>${Y?K:q`<span class="brand">ECU</span> Console`}</h1>
        <p>${X}</p>

        ${Y?q`
              <label for="code">Recovery code</label>
              <input id="code" type="text" autocomplete="off" spellcheck="false"
                placeholder="XXXX-XXXX-XXXX-XXXX" ?disabled=${this.busy} />
            `:J}

        <label for="pw">${Y||Q?"New password":"Password"}</label>
        <input id="pw" type="password"
          autocomplete=${Q||Y?"new-password":"current-password"}
          ?disabled=${this.busy} />

        ${Q||Y?q`
              <label for="pw2">Confirm password</label>
              <input id="pw2" type="password" autocomplete="new-password" ?disabled=${this.busy} />
            `:J}

        <button class="primary" type="submit" ?disabled=${this.busy}>
          ${this.busy?"…":Q?"Set password":Y?"Reset password":"Sign in"}
        </button>
        <div class="err">${this.error}</div>

        ${this.configured?q`<button class="link" type="button" @click=${this.toggleRecover}>
              ${Y?"Back to sign in":"Forgot password?"}
            </button>`:J}
      </form>
    `}toggleRecover=()=>{this.recoverMode=!this.recoverMode,this.error=""};renderSaved(){return q`
      <div class="box">
        <h1>Save your recovery code</h1>
        <p class="warn">
          Write this down and keep it safe. It's the only way to reset your password
          without console access, and it's shown only once. Using it later replaces it
          with a new code.
        </p>
        <div class="code">${this.savedCode}</div>
        <button class="copy" type="button" @click=${this.copyCode}>
          ${this.copied?"Copied ✓":"Copy to clipboard"}
        </button>
        <button class="primary" type="button" @click=${this.done}>
          I've saved it — continue
        </button>
      </div>
    `}}customElements.define("login-view",n3);function Z(Q){if(!Number.isFinite(Q))return"";return String(Number(Q.toFixed(3)))}function M(Q){if(!Number.isFinite(Q))return"—";if(Math.abs(Q)>=1000)return`${(Q/1000).toFixed(2)} kW`;return`${Math.round(Q)} W`}function I4(Q){if(!Number.isFinite(Q))return"—";let Y=Math.abs(Q);if(Y>=1e6)return`${(Q/1e6).toFixed(2)} MWh`;if(Y>=1000)return`${(Q/1000).toFixed(2)} kWh`;return`${Math.round(Q)} Wh`}function Q4(Q){return Number.isFinite(Q)?`${Q.toFixed(0)}%`:"—"}function V4(Q){return Q>0?`${Q.toFixed(1)} V`:"—"}function b4(Q){return Q>0?`${Q.toFixed(2)} Hz`:"—"}function i3(Q){return Number.isFinite(Q)?`${Q.toFixed(2)} A`:"—"}function C4(Q){if(!(Q>0))return"idle";if(Q<40)return"low";if(Q<85)return"mid";return"high"}function s3(Q){if(!Number.isFinite(Q)||Q<0)return"—";if(Q<60)return`${Math.round(Q)}s ago`;if(Q<3600)return`${Math.round(Q/60)}m ago`;return`${Math.round(Q/3600)}h ago`}function q3(Q){return Q.replace(/_/g," ").replace(/\b\w/g,(Y)=>Y.toUpperCase())}function k4(Q){if(!Q)return[];return Object.keys(Q).filter((Y)=>Q[Y]).map(q3)}function Y4(Q){if(!Q)return"—";return new Date(Q).toLocaleString(void 0,{hour12:!1})}function t3(Q){let Y=(Q||"").toLowerCase();if(Y==="error"||Y==="critical"||Y==="crit"||Y==="fault")return"err";if(Y==="warn"||Y==="warning")return"warn";return"info"}function h(Q){return Q.nameplate_w||0}function K4(Q){return Math.round(h(Q)*20/500)}function X4(Q){let Y=Q.protection?.DA;if(Y==null)return;return Math.round(Y/500*h(Q))}class a3 extends ${static properties={power:{type:Number},cap:{type:Number}};constructor(){super();this.power=0,this.cap=0}static styles=L`
    :host { display: block; text-align: center; }
    .wrap { position: relative; width: 220px; margin: 0 auto; }
    svg { width: 100%; height: auto; display: block; }
    .track { stroke: var(--bar-bg); }
    .arc { stroke-linecap: round; transition: stroke-dashoffset 0.5s ease, stroke 0.3s; }
    .arc.low { stroke: var(--ok); }
    .arc.mid { stroke: var(--accent); }
    .arc.high { stroke: var(--warn); }
    .arc.idle { stroke: var(--muted); }
    .center {
      position: absolute;
      left: 0;
      right: 0;
      bottom: 10%;
    }
    .big { font-size: 30px; font-weight: 700; color: var(--text); }
    .sub { font-size: 13px; color: var(--muted); margin-top: 2px; }
  `;pct(){if(!(this.cap>0))return 0;return Math.max(0,Math.min(100,this.power/this.cap*100))}render(){let Q=this.pct(),Y=C4(Q),K=90,X=Math.PI*90,G=X*(1-Q/100);return q`
      <div class="wrap">
        <svg viewBox="0 0 200 120" role="img" aria-label="fleet output gauge">
          <path
            class="track"
            d="M10 110 A 90 90 0 0 1 190 110"
            fill="none"
            stroke-width="14"
          />
          <path
            class="arc ${Y}"
            d="M10 110 A 90 90 0 0 1 190 110"
            fill="none"
            stroke-width="14"
            stroke-dasharray="${X}"
            stroke-dashoffset="${G}"
          />
        </svg>
        <div class="center">
          <div class="big">${M(this.power)}</div>
          <div class="sub">${Q4(Q)} of ${M(this.cap)}</div>
        </div>
      </div>
    `}}customElements.define("fleet-gauge",a3);class e3 extends ${static properties={label:{type:String},value:{type:String},sub:{type:String}};constructor(){super();this.label="",this.value="",this.sub=""}static styles=L`
    :host {
      display: block;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 14px 16px;
    }
    .label {
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    .value {
      font-size: 22px;
      font-weight: 700;
      color: var(--text);
      margin-top: 4px;
    }
    .sub {
      font-size: 12px;
      color: var(--muted);
      margin-top: 2px;
    }
  `;render(){return q`
      <div class="label">${this.label}</div>
      <div class="value">${this.value}</div>
      ${this.sub?q`<div class="sub">${this.sub}</div>`:""}
    `}}customElements.define("stat-card",e3);class Q9 extends ${static properties={inverter:{attribute:!1},pendingCap:{state:!0},busy:{state:!0},error:{state:!0}};dragging=!1;constructor(){super();this.pendingCap=null,this.busy=!1,this.error=""}static styles=L`
    :host { display: block; }
    .row { display: flex; align-items: center; gap: 10px; }
    .barwrap { position: relative; height: 20px; flex: 1; min-width: 90px; touch-action: none; cursor: pointer; }
    .barwrap.off { cursor: default; opacity: 0.6; }
    .bar { height: 8px; background: var(--bar-bg); border-radius: 4px; position: relative; overflow: hidden; }
    .fill { height: 100%; border-radius: 4px; transition: width 0.4s ease; }
    .fill.low { background: var(--ok); }
    .fill.mid { background: var(--accent); }
    .fill.high { background: var(--warn); }
    .fill.idle { background: var(--muted); }
    .capline { position: absolute; top: -1px; height: 10px; width: 2px; background: var(--err); transform: translateX(-1px); pointer-events: none; }
    .caret {
      position: absolute; top: 11px;
      width: 0; height: 0;
      border-left: 5px solid transparent;
      border-right: 5px solid transparent;
      border-bottom: 7px solid var(--err);
      transform: translateX(-5px);
      pointer-events: none;
    }
    .capval { color: var(--err); font-size: 13px; font-weight: 600; white-space: nowrap; font-variant-numeric: tabular-nums; }
    .caperr { color: var(--err); font-size: 12px; margin-top: 4px; }
  `;capFromEvent(Q){let Y=this.renderRoot.querySelector(".bar"),K=h(this.inverter);if(!Y)return this.pendingCap??K;let X=Y.getBoundingClientRect(),G=Math.max(0,Math.min(1,(Q.clientX-X.left)/X.width));return Math.min(K,Math.max(K4(this.inverter),Math.round(G*K)))}onDown=(Q)=>{if(!this.inverter?.online||this.busy)return;Q.preventDefault(),this.dragging=!0;try{Q.currentTarget.setPointerCapture?.(Q.pointerId)}catch{}this.pendingCap=this.capFromEvent(Q)};onMove=(Q)=>{if(this.dragging)this.pendingCap=this.capFromEvent(Q)};onUp=(Q)=>{if(!this.dragging)return;this.dragging=!1;try{Q.currentTarget.releasePointerCapture?.(Q.pointerId)}catch{}this.commitCap()};async commitCap(){let Q=this.pendingCap;if(Q==null)return;this.busy=!0,this.error="";try{let K=(await D.setPower({uid:this.inverter.uid,watts:Q})).results?.[0];if(K&&!K.ok)this.error=K.error||"failed";else if(K)this.pendingCap=K.applied_watts}catch(Y){this.error=Y.message||"failed"}finally{this.busy=!1}}render(){let Q=this.inverter;if(!Q)return J;let Y=C4(Q.load_pct),K=Math.max(0,Math.min(100,Q.load_pct)),X=h(Q);if(X<=0)return q`<div class="bar"><div class="fill ${Y}" style="width:${K}%"></div></div>`;let G=this.pendingCap??X4(Q)??X,z=Math.max(0,Math.min(100,G/X*100));return q`
      <div class="row">
        <div
          class="barwrap ${Q.online?"":"off"}"
          @pointerdown=${this.onDown}
          @pointermove=${this.onMove}
          @pointerup=${this.onUp}
          title="drag to set the output cap"
        >
          <div class="bar"><div class="fill ${Y}" style="width:${K}%"></div></div>
          <div class="capline" style="left:${z}%"></div>
          <div class="caret" style="left:${z}%"></div>
        </div>
        <span class="capval" title="output cap">▼ ${M(G)}</span>
      </div>
      ${this.error?q`<div class="caperr">⚠ ${this.error}</div>`:J}
    `}}customElements.define("cap-bar",Q9);class Y9 extends ${static properties={inverter:{attribute:!1},name:{type:String},profile:{type:String}};constructor(){super();this.name="",this.profile=""}static styles=L`
    :host {
      display: block;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 16px;
    }
    .head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 12px;
    }
    .model { font-weight: 600; font-size: 15px; }
    .uid { color: var(--muted); font-size: 12px; font-family: var(--mono); }
    .profile {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      margin-top: 6px;
      background: color-mix(in srgb, var(--accent) 16%, transparent);
      color: var(--accent);
      border: 1px solid color-mix(in srgb, var(--accent) 55%, transparent);
      border-radius: 999px;
      padding: 2px 9px;
      font-size: 11px;
      font-weight: 600;
    }
    .dot { width: 9px; height: 9px; border-radius: 50%; display: inline-block; margin-right: 6px; }
    .dot.on { background: var(--ok); box-shadow: 0 0 6px var(--ok); }
    .dot.off { background: var(--muted); }
    .state { font-size: 12px; color: var(--muted); }
    .power { display: flex; align-items: baseline; gap: 8px; }
    .pw { font-size: 28px; font-weight: 700; color: var(--text); }
    .cap { color: var(--muted); font-size: 13px; }
    cap-bar { margin: 10px 0 16px; }
    .metrics { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; font-size: 13px; }
    .metric .k { color: var(--muted); font-size: 11px; }
    .metric .v { color: var(--text); font-weight: 600; }
    .panels { margin-top: 14px; display: grid; grid-template-columns: repeat(auto-fill, minmax(76px, 1fr)); gap: 6px; }
    .panel { background: var(--bar-bg); border-radius: 6px; padding: 6px 8px; font-size: 11px; }
    .panel .pi { color: var(--muted); }
    .panel .pw { font-size: 13px; }
    .chips { margin-top: 12px; display: flex; flex-wrap: wrap; gap: 6px; }
    .chip {
      background: color-mix(in srgb, var(--err) 20%, transparent);
      color: var(--err);
      border: 1px solid var(--err);
      border-radius: 999px;
      padding: 2px 8px;
      font-size: 11px;
    }
  `;render(){let Q=this.inverter;if(!Q)return J;let Y=k4(Q.faults);return q`
      <div class="head">
        <div>
          <div class="model">${this.name||Q.model||"unknown"}</div>
          <div class="uid">${this.name?`${Q.model} · ${Q.uid}`:Q.uid}</div>
          ${this.profile?q`<div class="profile" title="Local Site profile active">⚙ ${this.profile}</div>`:J}
        </div>
        <div class="state">
          <span class="dot ${Q.online?"on":"off"}"></span>
          ${Q.online?"online":"offline"} · ${s3(Q.age_s)}
        </div>
      </div>

      <div class="power">
        <span class="pw">${M(Q.active_power_w)}</span>
        <span class="cap">/ ${M(Q.nameplate_w)} · ${Q4(Q.load_pct)}</span>
      </div>
      <cap-bar .inverter=${Q}></cap-bar>

      <div class="metrics">
        <div class="metric"><div class="k">Grid</div><div class="v">${V4(Q.grid_v)}</div></div>
        <div class="metric"><div class="k">Freq</div><div class="v">${b4(Q.freq_hz)}</div></div>
        <div class="metric"><div class="k">RSSI / LQI</div><div class="v">${Q.rssi} / ${Q.lqi}</div></div>
      </div>

      ${Q.panels?.length?q`<div class="panels">
            ${Q.panels.map((K)=>q`<div class="panel">
                <div class="pi">DC ${K.index+1}</div>
                <div class="pw">${M(K.w)}</div>
                <div>${V4(K.dc_v)} · ${i3(K.dc_a)}</div>
              </div>`)}
          </div>`:J}

      ${Y.length?q`<div class="chips">
            ${Y.map((K)=>q`<span class="chip">${K}</span>`)}
          </div>`:J}
    `}}customElements.define("inverter-card",Y9);class K9 extends ${static properties={system:{attribute:!1}};constructor(){super();this.system=null}static styles=L`
    :host { display: block; }
    .id {
      display: grid;
      grid-template-columns: auto 1fr;
      gap: 4px 12px;
      font-size: 13px;
      margin-bottom: 14px;
      padding-bottom: 14px;
      border-bottom: 1px solid var(--border);
    }
    .id .k { color: var(--muted); }
    .id .v { color: var(--text); font-family: var(--mono); }
    .peers { display: flex; flex-direction: column; gap: 8px; }
    .peer { display: flex; align-items: center; gap: 8px; font-size: 13px; }
    .dot { width: 9px; height: 9px; border-radius: 50%; flex: none; }
    .dot.on { background: var(--ok); box-shadow: 0 0 6px var(--ok); }
    .dot.off { background: var(--err); }
    .name { color: var(--text); flex: 1; }
    .role {
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      color: var(--muted);
      border: 1px solid var(--border);
      border-radius: 999px;
      padding: 1px 7px;
    }
    .ctl { color: var(--accent); border-color: var(--accent); }
    .ver { color: var(--muted); font-size: 11px; font-family: var(--mono); min-width: 0; }
    .warn { color: var(--warn); font-size: 12px; margin-top: 10px; }
    .empty { color: var(--muted); font-size: 13px; }
  `;idRow(Q,Y){return Y?q`<div class="k">${Q}</div><div class="v">${Y}</div>`:J}clients(){let Q=new Map;for(let Y of this.system?.peers??[]){let K=Q.get(Y.backend)??{backend:Y.backend,version:Y.version,controller:!1,conns:0};if(K.conns++,K.controller=K.controller||Y.controller,Y.version)K.version=Y.version;Q.set(Y.backend,K)}return[...Q.values()].sort((Y,K)=>Y.backend.localeCompare(K.backend))}render(){let Q=this.system,Y=Q?.ecu,K=this.clients(),X=!!(Y&&(Y.ecu_id||Y.hostname));return q`
      ${X?q`<div class="id">
            ${this.idRow("ECU ID",Y.ecu_id)}
            ${this.idRow("Host",Y.hostname)}
          </div>`:J}

      <div class="peers">
        ${K.length?K.map((G)=>q`<div class="peer">
                <span class="dot on"></span>
                <span class="name">${G.backend||"(unnamed)"}</span>
                ${G.controller?q`<span class="role ctl">ctrl</span>`:J}
                ${G.conns>1?q`<span class="role">${G.conns} conns</span>`:J}
                <span class="ver">${G.version||""}</span>
              </div>`):q`<div class="empty">No peers connected.</div>`}
      </div>

      ${Q?.status_error?q`<div class="warn">⚠ ${Q.status_error}</div>`:J}
    `}}customElements.define("ecu-clients-card",K9);function iQ(Q,Y,K){if(Q.length<2)return{line:"",area:"",max:0};let X=Q[0].t,G=Math.max(1,Q[Q.length-1].t-X),z=Math.max(1,...Q.map((A)=>A.w)),B=(A)=>[(A.t-X)/G*Y,K-A.w/z*K],H="";for(let A=0;A<Q.length;A++){let[F,I]=B(Q[A]);H+=`${A===0?"M":"L"}${F.toFixed(1)} ${I.toFixed(1)} `}let[W]=B(Q[0]),[U]=B(Q[Q.length-1]),j=`${H}L${U.toFixed(1)} ${K} L${W.toFixed(1)} ${K} Z`;return{line:H.trim(),area:j,max:z}}var x4=600,N4=160;class X9 extends ${static properties={points:{attribute:!1},hoverIdx:{state:!0}};constructor(){super();this.points=[],this.hoverIdx=-1}static styles=L`
    :host { display: block; }
    .empty { color: var(--muted); text-align: center; padding: 48px 0; font-size: 13px; }
    .wrap { position: relative; }
    svg { width: 100%; height: 160px; display: block; }
    .area { fill: url(#pc-grad); }
    .line { fill: none; stroke: var(--accent); stroke-width: 2; vector-effect: non-scaling-stroke; }
    .cross { stroke: var(--muted); stroke-width: 1; vector-effect: non-scaling-stroke; opacity: 0.6; }
    .cursor { fill: var(--accent); stroke: var(--bg); stroke-width: 1.5; vector-effect: non-scaling-stroke; }
    .tip {
      position: absolute;
      transform: translate(-50%, -118%);
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 4px 8px;
      font-size: 12px;
      color: var(--text);
      white-space: nowrap;
      pointer-events: none;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
    }
    .tip .t { color: var(--muted); }
    .tip .w { font-weight: 600; }
    .labels { display: flex; justify-content: space-between; font-size: 12px; color: var(--muted); margin-top: 6px; }
    .cur { color: var(--text); font-weight: 600; }
  `;onMove=(Q)=>{let Y=this.points.length;if(Y<2)return;let X=Q.currentTarget.clientWidth||1,G=Math.min(1,Math.max(0,Q.offsetX/X));this.hoverIdx=Math.round(G*(Y-1))};onLeave=()=>{this.hoverIdx=-1};render(){let Q=this.points??[];if(Q.length<2)return q`<div class="empty">Collecting power history…</div>`;let{line:Y,area:K,max:X}=iQ(Q,x4,N4),G=Q[Q.length-1].w,z=this.hoverIdx,B=z>=0&&z<Q.length,H=Q[0].t,W=Math.max(1,Q[Q.length-1].t-H),U=B?(Q[z].t-H)/W*x4:0,j=B?N4-Q[z].w/X*N4:0;return q`
      <div class="wrap">
        <svg
          viewBox="0 0 ${x4} ${N4}"
          preserveAspectRatio="none"
          role="img"
          aria-label="fleet output over time"
          @mousemove=${this.onMove}
          @mouseleave=${this.onLeave}
        >
          <defs>
            <linearGradient id="pc-grad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="var(--accent)" stop-opacity="0.35" />
              <stop offset="100%" stop-color="var(--accent)" stop-opacity="0" />
            </linearGradient>
          </defs>
          ${S`<path class="area" d=${K} />`}
          ${S`<path class="line" d=${Y} />`}
          ${B?S`<line class="cross" x1=${U} y1="0" x2=${U} y2=${N4} /><circle class="cursor" cx=${U} cy=${j} r="3.5" />`:J}
        </svg>
        ${B?q`<div class="tip" style="left:${U/x4*100}%; top:${j}px">
              <span class="w">${M(Q[z].w)}</span>
              <span class="t">· ${Y4(Q[z].t)}</span>
            </div>`:J}
      </div>
      <div class="labels">
        <span>now <span class="cur">${M(G)}</span></span>
        <span>peak ${M(X)}</span>
      </div>
    `}}customElements.define("power-chart",X9);class G9 extends ${static properties={fleet:{attribute:!1},system:{attribute:!1},names:{attribute:!1},profiles:{attribute:!1},history:{state:!0},arrayPendingCap:{state:!0},arrayBusy:{state:!0},arrayError:{state:!0}};timer=null;constructor(){super();this.fleet=null,this.system=null,this.names={},this.profiles={},this.history=[],this.arrayPendingCap=null,this.arrayBusy=!1,this.arrayError=""}setArrayCap=async(Q)=>{let Y=Math.round(Number(Q.target.value));if(!Number.isFinite(Y)||Y<=0)return;this.arrayPendingCap=Y,this.arrayBusy=!0,this.arrayError="";try{let K=await D.setPower({array:!0,watts:Y}),X=(K.results??[]).filter((G)=>!G.ok);if(X.length)this.arrayError=`${X.length} inverter(s) failed`;else{let G=(K.results??[]).reduce((z,B)=>z+B.applied_watts,0);if(G)this.arrayPendingCap=G}}catch(K){this.arrayError=K.message||"failed"}finally{this.arrayBusy=!1}};connectedCallback(){super.connectedCallback(),this.loadHistory(),this.timer=setInterval(()=>void this.loadHistory(),60000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async loadHistory(){try{this.history=await D.history()}catch{}}chartPoints(){if(!this.fleet)return this.history;return[...this.history,{t:Date.now(),w:this.fleet.active_power_w}]}static styles=L`
    :host { display: block; }
    .grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
      margin-bottom: 16px;
    }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 16px;
    }
    h2 { font-size: 13px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); margin: 0 0 14px; }
    .chart { margin-bottom: 16px; }
    .stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 16px; }
    .online { text-align: center; color: var(--muted); font-size: 12px; margin-top: 10px; }
    .arrcap { margin-top: 16px; }
    .arrcap label { display: block; color: var(--muted); font-size: 11px; margin-bottom: 6px; }
    .arrcap-row { display: flex; align-items: center; gap: 10px; }
    .arrcap input {
      width: 130px;
      box-sizing: border-box;
      padding: 9px 12px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font-size: 14px;
    }
    .arrcap input:disabled { opacity: 0.6; }
    .arrcap-max { color: var(--muted); font-size: 13px; }
    .caperr { color: var(--err); font-size: 12px; margin-top: 8px; }
    .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
    @media (max-width: 720px) { .grid, .stats { grid-template-columns: 1fr; } }
  `;render(){let Q=this.fleet;if(!Q)return q`<div class="empty">Waiting for inv-driver…</div>`;let Y=Q.inverters.reduce((z,B)=>z+h(B),0),K=Q.inverters.reduce((z,B)=>z+K4(B),0),X=Q.inverters.reduce((z,B)=>z+(X4(B)??h(B)),0),G=this.arrayPendingCap??X;return q`
      <div class="grid">
        <div class="panel">
          <h2>Array output</h2>
          <fleet-gauge .power=${Q.active_power_w} .cap=${Q.nameplate_total_w}></fleet-gauge>
          <div class="online">${Q.online_count} / ${Q.inverter_count} inverters online</div>
          ${Y>0?q`<div class="arrcap">
                <label for="arrcap">Total output cap</label>
                <div class="arrcap-row">
                  <input
                    id="arrcap"
                    type="number"
                    min=${K}
                    max=${Y}
                    step="10"
                    .value=${String(G)}
                    ?disabled=${Q.online_count===0||this.arrayBusy}
                    @change=${this.setArrayCap}
                  />
                  <span class="arrcap-max">W / ${M(Y)}</span>
                </div>
                ${this.arrayError?q`<div class="caperr">⚠ ${this.arrayError}</div>`:J}
              </div>`:J}
        </div>
        <div class="panel">
          <h2>ECU &amp; clients</h2>
          <ecu-clients-card .system=${this.system}></ecu-clients-card>
        </div>
      </div>

      <div class="panel chart">
        <h2>Output</h2>
        <power-chart .points=${this.chartPoints()}></power-chart>
      </div>

      <div class="stats">
        <stat-card label="Today" value=${I4(Q.today_wh)}></stat-card>
        <stat-card label="This month" value=${I4(Q.month_wh)}></stat-card>
        <stat-card label="This year" value=${I4(Q.year_wh)}></stat-card>
        <stat-card label="Lifetime" value=${I4(Q.lifetime_wh)}></stat-card>
      </div>

      <h2>Inverters</h2>
      ${Q.inverters.length?q`<div class="cards">
            ${Q.inverters.map((z)=>q`<inverter-card
                .inverter=${z}
                .name=${this.names?.[z.uid]??""}
                .profile=${this.profiles?.[z.uid]??""}
              ></inverter-card>`)}
          </div>`:q`<div class="empty">No inverters discovered yet.</div>`}
      ${J}
    `}}customElements.define("dashboard-view",G9);class q9 extends ${static properties={inverter:{attribute:!1},pendingCap:{state:!0},busy:{state:!0},error:{state:!0}};constructor(){super();this.pendingCap=null,this.busy=!1,this.error=""}static styles=L`
    :host { display: inline-block; }
    .row { display: flex; align-items: center; gap: 6px; white-space: nowrap; }
    input {
      width: 76px;
      box-sizing: border-box;
      padding: 5px 7px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      color: var(--text);
      font: inherit;
      text-align: right;
      font-variant-numeric: tabular-nums;
    }
    input:focus { outline: none; border-color: var(--accent); }
    input:disabled { opacity: 0.6; }
    .max { color: var(--muted); font-variant-numeric: tabular-nums; }
    .err { color: var(--err); }
  `;commit=async(Q)=>{let Y=Math.round(Number(Q.target.value));if(!Number.isFinite(Y)||Y<=0)return;this.pendingCap=Y,this.busy=!0,this.error="";try{let X=(await D.setPower({uid:this.inverter.uid,watts:Y})).results?.[0];if(X&&!X.ok)this.error=X.error||"failed";else if(X)this.pendingCap=X.applied_watts}catch(K){this.error=K.message||"failed"}finally{this.busy=!1}};render(){let Q=this.inverter;if(!Q)return J;let Y=h(Q);if(Y<=0)return q`<span class="max">—</span>`;let K=this.pendingCap??X4(Q)??Y;return q`
      <div class="row">
        <input
          type="number"
          min=${K4(Q)}
          max=${Y}
          step="10"
          .value=${String(Math.round(K))}
          ?disabled=${!Q.online||this.busy}
          @change=${this.commit}
          title="output cap, watts"
        />
        <span class="max">/ ${Math.round(Y)} W</span>
        ${this.error?q`<span class="err" title=${this.error}>⚠</span>`:J}
      </div>
    `}}customElements.define("cap-input",q9);class z9 extends ${static properties={busy:{attribute:!1},slow:{state:!0},serial:{state:!0}};constructor(){super();this.busy=!1,this.slow=!1,this.serial=""}static styles=L`
    :host { display: block; }
    .panel { display: grid; gap: 16px; max-width: 520px; }
    fieldset { border: 1px solid var(--border); border-radius: 10px; padding: 14px 16px; margin: 0; }
    legend { color: var(--muted); font-size: 12px; padding: 0 6px; text-transform: uppercase; letter-spacing: 0.04em; }
    .row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
    .toggle { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
    .toggle button {
      background: transparent; border: none; color: var(--muted);
      padding: 7px 16px; font: inherit; font-size: 13px; cursor: pointer;
    }
    .toggle button.sel { background: var(--accent); color: #04121a; font-weight: 600; }
    .warn {
      color: var(--err); font-size: 12px; background: color-mix(in srgb, var(--err) 12%, transparent);
      border: 1px solid color-mix(in srgb, var(--err) 40%, transparent);
      border-radius: 8px; padding: 8px 10px;
    }
    input.serial {
      background: var(--bar-bg); border: 1px solid var(--border); color: var(--text);
      border-radius: 8px; padding: 8px 11px; font: inherit; font-size: 14px;
      font-family: var(--mono); width: 170px; letter-spacing: 0.02em;
    }
    input.serial:focus { outline: none; border-color: var(--accent); }
    button.go {
      background: var(--accent); border: none; color: #04121a; border-radius: 8px;
      padding: 8px 18px; font-size: 14px; font-weight: 600; cursor: pointer; white-space: nowrap;
    }
    button.go:hover:not(:disabled) { filter: brightness(1.08); }
    button.go:disabled { opacity: 0.45; cursor: not-allowed; }
    .hint { font-size: 12px; color: var(--muted); }
  `;startScan=()=>{if(this.busy)return;this.dispatchEvent(new CustomEvent("scan",{detail:{slow:this.slow},bubbles:!0,composed:!0}))};onSerialInput=(Q)=>{this.serial=Q.target.value.replace(/\D/g,"").slice(0,12)};addById=()=>{if(this.busy||this.serial.length!==12)return;this.dispatchEvent(new CustomEvent("add",{detail:{serial:this.serial},bubbles:!0,composed:!0})),this.serial=""};render(){let Q=this.serial.length===12;return q`
      <div class="panel">
        <fieldset>
          <legend>Scan for inverters</legend>
          <div class="row">
            <div class="toggle" role="group" aria-label="Scan speed">
              <button
                class=${!this.slow?"sel":""}
                aria-pressed=${!this.slow}
                ?disabled=${this.busy}
                @click=${()=>this.slow=!1}
              >Fast</button>
              <button
                class=${this.slow?"sel":""}
                aria-pressed=${this.slow}
                ?disabled=${this.busy}
                @click=${()=>this.slow=!0}
              >Slow</button>
            </div>
            <button class="go" ?disabled=${this.busy} @click=${this.startScan}>
              ${this.busy?"Scanning…":"Scan"}
            </button>
          </div>
          ${this.slow?q`<p class="warn" role="alert">
                Slow scan sweeps the radio across ZigBee channels 11–26 on PAN 0xFFFF.
                This pauses telemetry for ~30 seconds while the module is off the
                operating PAN. Use for commissioning only.
              </p>`:q`<p class="hint">
                Fast scan solicits new inverters on PAN 0xFFFF on the current channel.
                Telemetry briefly pauses while the radio is parked for discovery.
              </p>`}
        </fieldset>

        <fieldset>
          <legend>Add by serial</legend>
          <div class="row">
            <input
              class="serial"
              inputmode="numeric"
              placeholder="12-digit serial"
              .value=${this.serial}
              ?disabled=${this.busy}
              @input=${this.onSerialInput}
              @keydown=${(Y)=>{if(Y.key==="Enter")this.addById()}}
            />
            <button class="go" ?disabled=${this.busy||!Q} @click=${this.addById}>Add</button>
          </div>
          ${this.serial.length>0&&!Q?q`<p class="hint">Serial must be exactly 12 digits (${this.serial.length}/12).</p>`:J}
        </fieldset>
      </div>
    `}}customElements.define("pairing-scan-panel",z9);var z3=["scan","bind","migrate","configure","rekey"],B9={scan:"Scan",bind:"Bind",migrate:"Migrate",configure:"Configure",rekey:"Re-key",done:"Done",aborted:"Aborted",error:"Error"};class J9 extends ${static properties={open:{attribute:!1},status:{attribute:!1},aborting:{attribute:!1}};constructor(){super();this.open=!1,this.status=null,this.aborting=!1}static styles=L`
    :host { display: block; }
    .scrim {
      position: fixed; inset: 0; background: rgba(0, 0, 0, 0.45);
      z-index: 40; display: none;
    }
    .scrim.open { display: block; }
    .drawer {
      position: fixed; top: 0; right: 0; bottom: 0; width: 420px; max-width: 92vw;
      background: var(--bg, #0c1116); border-left: 1px solid var(--border);
      box-shadow: -8px 0 30px rgba(0, 0, 0, 0.4);
      transform: translateX(100%); transition: transform 0.18s ease;
      z-index: 41; display: flex; flex-direction: column;
      box-sizing: border-box;
    }
    .scrim.open .drawer { transform: translateX(0); }
    header {
      display: flex; align-items: center; justify-content: space-between;
      padding: 16px 18px; border-bottom: 1px solid var(--border);
    }
    header h2 { margin: 0; font-size: 16px; color: var(--text); }
    button.x {
      background: transparent; border: 1px solid var(--border); color: var(--muted);
      border-radius: 8px; padding: 4px 10px; font-size: 15px; line-height: 1; cursor: pointer;
    }
    button.x:disabled { opacity: 0.4; cursor: not-allowed; }
    .body { padding: 16px 18px; overflow-y: auto; display: grid; gap: 16px; }
    .stages { display: flex; gap: 6px; flex-wrap: wrap; }
    .stage {
      font-size: 11px; padding: 4px 9px; border-radius: 999px;
      border: 1px solid var(--border); color: var(--muted);
    }
    .stage.active { background: var(--accent); color: #04121a; border-color: var(--accent); font-weight: 600; }
    .stage.done { color: var(--ok); border-color: color-mix(in srgb, var(--ok) 50%, transparent); }
    .bar { height: 8px; border-radius: 999px; background: var(--bar-bg); overflow: hidden; }
    .bar > i { display: block; height: 100%; background: var(--accent); transition: width 0.2s ease; }
    .meta { font-size: 13px; color: var(--text); display: grid; gap: 4px; }
    .meta .muted { color: var(--muted); }
    .sweep { font-size: 12px; color: var(--muted); }
    .err { color: var(--err); font-size: 13px; }
    .ok { color: var(--ok); font-size: 13px; }
    table { width: 100%; border-collapse: collapse; font-size: 12px; }
    th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--border); }
    th { color: var(--muted); text-transform: uppercase; font-size: 10px; letter-spacing: 0.04em; }
    td.mono { font-family: var(--mono); }
    .actions { padding: 14px 18px; border-top: 1px solid var(--border); display: flex; gap: 12px; }
    button.abort {
      background: transparent; border: 1px solid var(--err); color: var(--err);
      border-radius: 8px; padding: 8px 16px; font-size: 13px; font-weight: 600; cursor: pointer;
    }
    button.abort:hover:not(:disabled) { background: color-mix(in srgb, var(--err) 14%, transparent); }
    button.abort:disabled { opacity: 0.4; cursor: not-allowed; }
    .empty { color: var(--muted); font-size: 13px; }
    @media (max-width: 480px) { .drawer { width: 100vw; } }
  `;abort=()=>{this.dispatchEvent(new CustomEvent("abort",{bubbles:!0,composed:!0}))};close=()=>{this.dispatchEvent(new CustomEvent("close",{bubbles:!0,composed:!0}))};stageClass(Q,Y){if(Q===Y)return"stage active";let K=z3.indexOf(Q);if(z3.indexOf(Y)>K&&K>=0)return"stage done";return"stage"}render(){let Q=this.status,Y=e(Q),K=Q?.stage??"",X=Q?.total??0,G=Q?.done??0,z=X>0?Math.min(100,Math.round(G/X*100)):Y?0:0;return q`
      <div class="scrim ${this.open?"open":""}" @click=${(B)=>{if(B.target===B.currentTarget&&!Y)this.close()}}>
        <aside class="drawer" role="dialog" aria-label="Pairing progress" aria-modal="true">
          <header>
            <h2>${Q?.op?`Pairing: ${Q.op}`:"Pairing"}</h2>
            <button class="x" aria-label="Close" ?disabled=${Y} @click=${this.close}>✕</button>
          </header>
          <div class="body">
            ${!Q||!Q.op?q`<p class="empty">No pairing operation running.</p>`:q`
                  <div class="stages">
                    ${z3.map((B)=>q`<span class=${this.stageClass(B,K)}>${B9[B]}</span>`)}
                  </div>

                  ${X>0?q`<div class="bar"><i style="width:${z}%"></i></div>
                        <div class="meta"><span class="muted">${G} / ${X} inverters</span></div>`:J}

                  <div class="meta">
                    <div><span class="muted">Stage:</span> ${B9[K]??K??"—"}</div>
                    ${Q.current_serial?q`<div><span class="muted">Current:</span> ${Q.current_serial}</div>`:J}
                    ${Q.substep?q`<div><span class="muted">Step:</span> ${Q.substep}</div>`:J}
                    ${Q.message?q`<div class="muted">${Q.message}</div>`:J}
                  </div>

                  ${Q.sweep?q`<div class="sweep">Channel ${Q.sweep.chan} (sweep ${Q.sweep.chan_lo}–${Q.sweep.chan_hi}) — telemetry paused</div>`:J}

                  ${Q.error?q`<div class="err">Error: ${Q.error}</div>`:J}
                  ${K==="done"?q`<div class="ok">Completed.</div>`:J}
                  ${K==="aborted"?q`<div class="muted">Aborted.</div>`:J}

                  ${Q.per_inverter&&Q.per_inverter.length>0?q`<table>
                        <thead><tr><th>Serial</th><th>Addr</th><th>State</th><th>Link</th></tr></thead>
                        <tbody>
                          ${Q.per_inverter.map((B)=>q`<tr>
                              <td class="mono">${B.serial}</td>
                              <td class="mono">${B.short_addr?B.short_addr.toString(16):"—"}</td>
                              <td>${B.state}</td>
                              <td>${B.encrypted===!0?"\uD83D\uDD12":B.encrypted===!1?"⚠":"—"}</td>
                            </tr>`)}
                        </tbody>
                      </table>`:J}
                `}
          </div>
          <div class="actions">
            <button class="abort" ?disabled=${!Y||this.aborting} @click=${this.abort}>
              ${this.aborting?"Aborting…":"Safe abort"}
            </button>
          </div>
        </aside>
      </div>
    `}}customElements.define("pairing-progress-drawer",J9);class H9 extends ${static properties={kind:{attribute:!0},busy:{attribute:!1},actionError:{attribute:!1},value:{state:!0},password:{state:!0},pwdError:{state:!0},valueError:{state:!0},pwdBusy:{state:!0}};constructor(){super();this.kind="rekey",this.busy=!1,this.actionError="",this.value="",this.password="",this.pwdError="",this.valueError="",this.pwdBusy=!1}static styles=L`
    :host { display: contents; }
    .backdrop {
      position: fixed; inset: 0;
      background: rgba(0, 0, 0, 0.55);
      display: flex; align-items: center; justify-content: center;
      z-index: 1000;
    }
    .dialog {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 20px 22px;
      max-width: 460px;
      width: 92%;
      color: var(--text);
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
      box-sizing: border-box;
    }
    .dialog h3 { margin: 0 0 10px; font-size: 15px; }
    .dialog p {
      margin: 0 0 14px; font-size: 13px; color: var(--muted); line-height: 1.45;
    }
    .dialog p.warn {
      color: var(--text);
      border: 1px solid var(--accent);
      background: color-mix(in srgb, var(--accent) 10%, transparent);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      margin: 0 0 12px;
    }
    .dialog label {
      display: block; font-size: 12px; color: var(--muted);
      margin: 8px 0 6px;
    }
    .dialog input {
      width: 100%;
      box-sizing: border-box;
      padding: 9px 11px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font: inherit;
    }
    .dialog .err {
      color: var(--err);
      border: 1px solid var(--err);
      background: color-mix(in srgb, var(--err) 12%, transparent);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      margin-top: 10px;
    }
    .dialog .err-inline {
      color: var(--err); font-size: 12px; margin-top: 6px;
    }
    .dialog .row {
      display: flex; gap: 10px; justify-content: flex-end; margin-top: 16px;
    }
    .dialog button {
      padding: 8px 14px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    .dialog button.primary {
      background: var(--accent); color: #04121a; border: none;
    }
    .dialog button.secondary {
      background: transparent; color: var(--muted); border: 1px solid var(--border);
    }
    .dialog button:disabled { opacity: 0.6; cursor: default; }
    @media (max-width: 480px) {
      .dialog { width: 96%; padding: 18px 16px; }
    }
  `;firstUpdated(){queueMicrotask(()=>{this.shadowRoot?.querySelector("#pcd_value")?.focus()})}validateValue(Q){let Y=Q.trim();if(this.kind==="rekey"){if(!/^[0-9a-fA-F]{1,4}$/.test(Y))return"PAN must be 1–4 hexadecimal digits.";return""}let K=Number(Y);if(!Number.isInteger(K)||K<11||K>26)return"Channel must be an integer 11–26.";return""}onValueInput=(Q)=>{if(this.value=Q.target.value,this.valueError)this.valueError=""};onPasswordInput=(Q)=>{if(this.password=Q.target.value,this.pwdError)this.pwdError=""};onKey=(Q)=>{if(Q.key==="Enter")Q.preventDefault(),this.submit();else if(Q.key==="Escape")Q.preventDefault(),this.cancel()};async submit(){if(this.pwdBusy||this.busy)return;let Q=this.validateValue(this.value);if(Q){this.valueError=Q,this.shadowRoot?.querySelector("#pcd_value")?.focus();return}if(!this.password){this.pwdError="Password required.",this.shadowRoot?.querySelector("#pcd_pwd")?.focus();return}this.pwdBusy=!0,this.pwdError="";try{if(!await D.verifyPassword(this.password)){this.pwdError="Password is wrong.",this.shadowRoot?.querySelector("#pcd_pwd")?.focus();return}let K={value:this.value.trim()};this.dispatchEvent(new CustomEvent("confirm",{detail:K,bubbles:!0,composed:!0}))}catch(Y){this.pwdError=Y.message||"Verification failed."}finally{this.pwdBusy=!1}}cancel=()=>{if(this.pwdBusy||this.busy)return;this.dispatchEvent(new CustomEvent("cancel",{bubbles:!0,composed:!0}))};onBackdrop=(Q)=>{if(Q.target===Q.currentTarget)this.cancel()};stop=(Q)=>Q.stopPropagation();renderRekey(){return q`
      <h3>Fleet re-key</h3>
      <p>
        Broadcasts a new PAN to every inverter (opcode 0x22) and moves the
        radio onto it. Telemetry pauses while the broadcast runs; on failure
        the old PAN is restored.
      </p>
      <p class="warn">
        Privileged action — your password is required to confirm.
      </p>
      <label for="pcd_value">New PAN (1–4 hex digits, e.g. 0DCE)</label>
      <input
        id="pcd_value"
        type="text"
        autocomplete="off"
        spellcheck="false"
        maxlength="4"
        placeholder="0DCE"
        .value=${this.value}
        @input=${this.onValueInput}
        @keydown=${this.onKey}
        ?disabled=${this.pwdBusy||this.busy}
      />
      ${this.valueError?q`<div class="err-inline">${this.valueError}</div>`:J}
    `}renderChannel(){return q`
      <h3>Change ZigBee channel</h3>
      <p>
        Migrates the whole fleet to a new RF channel: each inverter is hopped
        to the new channel, then the radio follows. Telemetry pauses while the
        radio moves.
      </p>
      <p class="warn">
        Not atomic — an inverter hops the instant it gets the command, so a
        partway failure can split the fleet across the old and new channels
        (the module rolls back, but already-hopped units stay on the new one).
        Re-running this same change-channel toward the new channel converges
        them. Privileged action — your password is required to confirm.
      </p>
      <label for="pcd_value">New channel (11–26)</label>
      <input
        id="pcd_value"
        type="number"
        min="11"
        max="26"
        step="1"
        inputmode="numeric"
        placeholder="20"
        .value=${this.value}
        @input=${this.onValueInput}
        @keydown=${this.onKey}
        ?disabled=${this.pwdBusy||this.busy}
      />
      ${this.valueError?q`<div class="err-inline">${this.valueError}</div>`:J}
    `}render(){let Q=this.kind==="rekey"?"Re-key fleet":"Change channel";return q`
      <div class="backdrop" @click=${this.onBackdrop}>
        <div class="dialog" role="dialog" aria-modal="true" @click=${this.stop}>
          ${this.kind==="rekey"?this.renderRekey():this.renderChannel()}
          <label for="pcd_pwd">Password</label>
          <input
            id="pcd_pwd"
            type="password"
            autocomplete="current-password"
            .value=${this.password}
            @input=${this.onPasswordInput}
            @keydown=${this.onKey}
            ?disabled=${this.pwdBusy||this.busy}
          />
          ${this.pwdError?q`<div class="err">${this.pwdError}</div>`:J}
          ${this.actionError&&!this.pwdError?q`<div class="err">${this.actionError}</div>`:J}
          <div class="row">
            <button
              class="secondary"
              type="button"
              @click=${this.cancel}
              ?disabled=${this.pwdBusy||this.busy}
            >
              Cancel
            </button>
            <button
              class="primary"
              type="button"
              @click=${()=>void this.submit()}
              ?disabled=${this.pwdBusy||this.busy}
            >
              ${this.pwdBusy?"Verifying…":this.busy?"Working…":Q}
            </button>
          </div>
        </div>
      </div>
    `}}customElements.define("password-confirm-dialog",H9);class W9 extends ${static properties={fleet:{attribute:!1},names:{attribute:!1},status:{state:!0},drawerOpen:{state:!0},busy:{state:!0},aborting:{state:!0},notice:{state:!0},privilegedDialog:{state:!0},privilegedError:{state:!0}};pollTimer=null;constructor(){super();this.fleet=null,this.names={},this.status=null,this.drawerOpen=!1,this.busy=!1,this.aborting=!1,this.notice="",this.privilegedDialog="",this.privilegedError=""}connectedCallback(){super.connectedCallback(),this.fetchStatus()}disconnectedCallback(){super.disconnectedCallback(),this.stopPoll()}rename(Q,Y){let K=Y.target.value;this.dispatchEvent(new CustomEvent("rename",{detail:{uid:Q,name:K},bubbles:!0,composed:!0}))}encBadge(Q){if(Q===!0)return q`<span class="enc enc-ok" title="AES-encrypted link">🔒 AES</span>`;if(Q===!1)return q`<span class="enc enc-warn" title="Plaintext link — misconfigured or foreign unit">⚠ plaintext</span>`;return q`<span class="enc enc-unknown" title="Encryption state unknown">—</span>`}async fetchStatus(){try{let Q=await D.pairingStatus();if(this.status=Q.status??null,e(this.status))this.drawerOpen=!0,this.startPoll();else this.stopPoll()}catch{}}startPoll(){if(this.pollTimer)return;this.pollTimer=setInterval(()=>void this.fetchStatus(),1000)}stopPoll(){if(this.pollTimer)clearInterval(this.pollTimer);this.pollTimer=null}applyResp(Q){if(this.status=Q??null,this.drawerOpen=!0,e(this.status))this.startPoll()}onScan=async(Q)=>{let{slow:Y}=Q.detail;if(Y&&!confirm("Slow scan sweeps ZigBee channels 11–26 on PAN 0xFFFF and pauses fleet "+"telemetry for ~30 seconds. Continue?"))return;this.busy=!0,this.notice="";try{let K=await D.pairingScan({slow:Y});if(!K.ok)throw Error(K.error||"scan rejected");this.applyResp(K.status)}catch(K){this.notice=String(K.message||K)}finally{this.busy=!1}};onAdd=async(Q)=>{let{serial:Y}=Q.detail;this.busy=!0,this.notice="";try{let K=await D.pairingAdd(Y);if(!K.ok)throw Error(K.error||"add rejected");this.applyResp(K.status)}catch(K){this.notice=String(K.message||K)}finally{this.busy=!1}};onReplace=async(Q)=>{let Y=prompt(`Replace inverter ${Q}.

Enter the replacement's 12-digit serial, or leave blank to scan for it. The new unit inherits this one's grid profile, power cap and array slot.`);if(Y===null)return;let K=Y.replace(/\D/g,"");if(K!==""&&K.length!==12){this.notice="Replacement serial must be 12 digits (or blank to scan).";return}this.busy=!0,this.notice="";try{let X=await D.pairingReplace(Q,K);if(!X.ok)throw Error(X.error||"replace rejected");this.applyResp(X.status)}catch(X){this.notice=String(X.message||X)}finally{this.busy=!1}};onRekey=()=>{this.notice="",this.privilegedError="",this.privilegedDialog="rekey"};onChangeChannel=()=>{this.notice="",this.privilegedError="",this.privilegedDialog="channel"};onPrivilegedCancel=()=>{if(this.busy)return;this.privilegedDialog="",this.privilegedError=""};onPrivilegedConfirm=async(Q)=>{let Y=this.privilegedDialog;if(!Y)return;let{value:K}=Q.detail;this.busy=!0,this.privilegedError="",this.notice="";try{let X=Y==="rekey"?await D.pairingRekey(K,0):await D.pairingChangeChannel(Number(K));if(!X.ok){let G=X.error||(Y==="rekey"?"re-key rejected":"channel change rejected");throw Error(G)}this.privilegedDialog="",this.privilegedError="",this.applyResp(X.status)}catch(X){this.privilegedError=String(X.message||X)}finally{this.busy=!1}};onAbort=async()=>{this.aborting=!0;try{let Q=await D.pairingAbort();this.status=Q.status??this.status}catch(Q){this.notice=String(Q.message||Q)}finally{this.aborting=!1,this.fetchStatus()}};onCloseDrawer=()=>{if(e(this.status))return;this.drawerOpen=!1};static styles=L`
    :host { display: block; }
    .controls {
      display: flex; align-items: flex-start; justify-content: space-between;
      gap: 16px; flex-wrap: wrap; margin-bottom: 20px;
    }
    .rekey {
      display: flex; flex-direction: column; gap: 6px; align-items: flex-end;
    }
    button.rekey-btn {
      background: transparent; border: 1px solid var(--err); color: var(--err);
      border-radius: 8px; padding: 8px 16px; font-size: 13px; font-weight: 600; cursor: pointer;
      white-space: nowrap;
    }
    button.rekey-btn:hover:not(:disabled) { background: color-mix(in srgb, var(--err) 12%, transparent); }
    button.rekey-btn:disabled { opacity: 0.45; cursor: not-allowed; }
    .rekey .hint { font-size: 11px; color: var(--muted); max-width: 220px; text-align: right; }
    .notice {
      color: var(--err); font-size: 13px; margin-bottom: 16px;
      border: 1px solid color-mix(in srgb, var(--err) 40%, transparent);
      border-radius: 8px; padding: 8px 10px;
    }
    .table-wrap { overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid var(--border); }
    th { color: var(--muted); text-transform: uppercase; font-size: 11px; letter-spacing: 0.04em; }
    td { color: var(--text); }
    .uid { font-family: var(--mono); color: var(--muted); font-size: 11px; }
    .name-in {
      background: transparent;
      border: 1px solid transparent;
      border-radius: 6px;
      color: var(--text);
      font: inherit;
      padding: 3px 6px;
      width: 150px;
    }
    .name-in:hover { border-color: var(--border); }
    .name-in:focus { outline: none; border-color: var(--accent); background: var(--bar-bg); }
    .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; margin-right: 6px; }
    .dot.on { background: var(--ok); }
    .dot.off { background: var(--muted); }
    .num { text-align: right; font-variant-numeric: tabular-nums; }
    .capcell { white-space: nowrap; }
    .fw { font-variant-numeric: tabular-nums; color: var(--muted); }
    .fault { color: var(--err); }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
    .enc { font-size: 11px; white-space: nowrap; }
    .enc-ok { color: var(--ok); }
    .enc-warn { color: var(--err); }
    .enc-unknown { color: var(--muted); }
    button.replace {
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 6px;
      padding: 4px 10px;
      font-size: 12px;
      cursor: pointer;
      white-space: nowrap;
    }
    button.replace:hover { color: var(--text); border-color: var(--muted); }
  `;renderTable(){let Q=this.fleet;if(!Q||Q.inverters.length===0)return q`<div class="empty">No inverters discovered yet.</div>`;return q`
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Inverter ID</th><th>Name</th><th>Model</th><th>Firmware</th><th>Status</th>
              <th>Encryption</th>
              <th class="num">Output</th><th class="num">Load</th><th>Output cap</th>
              <th class="num">Grid</th><th class="num">Freq</th>
              <th class="num">Panels</th><th class="num">Faults</th><th></th>
            </tr>
          </thead>
          <tbody>
            ${Q.inverters.map((Y)=>{let K=Y.faults?Object.values(Y.faults).filter(Boolean).length:0;return q`<tr>
                <td class="uid">${Y.uid}</td>
                <td>
                  <input
                    class="name-in"
                    .value=${this.names?.[Y.uid]??""}
                    placeholder="add a name"
                    @change=${(X)=>this.rename(Y.uid,X)}
                  />
                </td>
                <td>${Y.model||"—"}</td>
                <td class="fw">${Y.sw_version||"—"}</td>
                <td>
                  <span class="dot ${Y.online?"on":"off"}"></span>${Y.online?"online":"offline"}
                </td>
                <td>${this.encBadge(Y.encrypted)}</td>
                <td class="num">${M(Y.active_power_w)} / ${M(Y.nameplate_w)}</td>
                <td class="num">${Q4(Y.load_pct)}</td>
                <td class="capcell"><cap-input .inverter=${Y}></cap-input></td>
                <td class="num">${V4(Y.grid_v)}</td>
                <td class="num">${b4(Y.freq_hz)}</td>
                <td class="num">${Y.panels?.length??0}</td>
                <td class="num ${K?"fault":""}">${K||"—"}</td>
                <td>
                  <button class="replace" title="Replace this inverter with a new unit"
                    ?disabled=${this.busy}
                    @click=${()=>this.onReplace(Y.uid)}>Replace</button>
                </td>
              </tr>`})}
          </tbody>
        </table>
      </div>
    `}render(){return q`
      <div class="controls">
        <pairing-scan-panel
          .busy=${this.busy}
          @scan=${this.onScan}
          @add=${this.onAdd}
        ></pairing-scan-panel>
        <div class="rekey">
          <button class="rekey-btn" ?disabled=${this.busy} @click=${this.onRekey}>Fleet re-key…</button>
          <span class="hint">Broadcasts a new PAN to the whole fleet. Confirmation required.</span>
          <button class="rekey-btn" ?disabled=${this.busy} @click=${this.onChangeChannel}>Change ZigBee channel…</button>
          <span class="hint">Migrates the whole fleet to a new RF channel. Confirmation required.</span>
        </div>
      </div>

      ${this.notice?q`<div class="notice" role="alert">${this.notice}</div>`:J}

      ${this.renderTable()}

      <pairing-progress-drawer
        .open=${this.drawerOpen}
        .status=${this.status}
        .aborting=${this.aborting}
        @abort=${this.onAbort}
        @close=${this.onCloseDrawer}
      ></pairing-progress-drawer>

      ${this.privilegedDialog?q`<password-confirm-dialog
            .kind=${this.privilegedDialog}
            .busy=${this.busy}
            .actionError=${this.privilegedError}
            @confirm=${this.onPrivilegedConfirm}
            @cancel=${this.onPrivilegedCancel}
          ></password-confirm-dialog>`:J}
    `}}customElements.define("inverters-view",W9);class U9 extends ${static properties={events:{attribute:!1}};constructor(){super();this.events=[]}static styles=L`
    :host { display: block; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { text-align: left; padding: 9px 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
    th { color: var(--muted); text-transform: uppercase; font-size: 11px; letter-spacing: 0.04em; }
    td { color: var(--text); }
    .time { color: var(--muted); white-space: nowrap; font-variant-numeric: tabular-nums; }
    .uid { font-family: var(--mono); color: var(--muted); font-size: 12px; }
    .by { color: var(--accent); font-size: 12px; white-space: nowrap; }
    .detail { color: var(--muted); }
    .sev {
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      border-radius: 999px;
      padding: 1px 8px;
      border: 1px solid var(--border);
    }
    .sev.info { color: var(--muted); }
    .sev.warn { color: var(--warn); border-color: var(--warn); }
    .sev.err { color: var(--err); border-color: var(--err); }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
  `;render(){if(!this.events||this.events.length===0)return q`<div class="empty">No events recorded.</div>`;return q`
      <table>
        <thead>
          <tr><th>Time</th><th>Severity</th><th>Event</th><th>By</th><th>Inverter</th><th>Detail</th></tr>
        </thead>
        <tbody>
          ${this.events.map((Q)=>q`<tr>
              <td class="time">${Y4(Q.ts_ms)}</td>
              <td><span class="sev ${t3(Q.severity)}">${Q.severity}</span></td>
              <td>${q3(Q.kind)}</td>
              <td class="by">${Q.by||"—"}</td>
              <td class="uid">${Q.inverter_uid||"—"}</td>
              <td class="detail">${Q.detail||(Q.raw_hex?Q.raw_hex:J)}</td>
            </tr>`)}
        </tbody>
      </table>
    `}}customElements.define("events-table",U9);var sQ=30000,tQ=86400000,aQ=100;class j9 extends ${static properties={fleet:{attribute:!1},recent:{state:!0},recentLoading:{state:!0},recentError:{state:!0}};timer=null;constructor(){super();this.fleet=null,this.recent=[],this.recentLoading=!1,this.recentError=""}static styles=L`
    :host { display: block; }
    .row {
      display: flex;
      align-items: center;
      gap: 12px;
      background: var(--surface);
      border: 1px solid var(--border);
      border-left-width: 3px;
      border-radius: 8px;
      padding: 12px 14px;
      margin-bottom: 8px;
    }
    .row.fault { border-left-color: var(--err); }
    .row.warning { border-left-color: var(--warn); }
    .sev {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      width: 64px;
    }
    .row.fault .sev { color: var(--err); }
    .row.warning .sev { color: var(--warn); }
    .label { color: var(--text); flex: 1; }
    .uid { font-family: var(--mono); color: var(--muted); font-size: 12px; }
    .ok { color: var(--muted); padding: 32px; text-align: center; }
    .ok .big { color: var(--ok); font-size: 16px; }

    .section { margin-top: 24px; }
    .section h3 {
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--muted);
      margin: 0 0 10px 2px;
      font-weight: 600;
    }
    .section .err { color: var(--muted); font-size: 12px; margin: 0 2px 8px; }
    .panel { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; overflow: hidden; }
    .empty { color: var(--muted); padding: 24px; text-align: center; font-size: 13px; }
  `;connectedCallback(){super.connectedCallback(),this.loadRecent(),this.timer=setInterval(()=>void this.loadRecent(),sQ)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer),this.timer=null}updated(Q){if(Q.has("fleet"))this.loadRecent()}async loadRecent(){this.recentLoading=!0;try{let Q=await D.events({kind:"fault_raised",since_ms:Date.now()-tQ,limit:aQ});this.recent=Q.events??[],this.recentError=Q.error??""}catch(Q){this.recentError=Q.message||"failed to load events"}finally{this.recentLoading=!1}}alarms(){let Q=[];for(let Y of this.fleet?.inverters??[]){for(let K of k4(Y.faults))Q.push({uid:Y.uid,model:Y.model,label:K,severity:"fault"});if(!Y.online)Q.push({uid:Y.uid,model:Y.model,label:"Inverter offline",severity:"warning"})}return Q}renderLive(){let Q=this.alarms();if(Q.length===0)return q`<div class="ok"><div class="big">✓ No active alarms</div><div>All inverters reporting healthy.</div></div>`;return q`${Q.map((Y)=>q`<div class="row ${Y.severity}">
        <span class="sev">${Y.severity}</span>
        <span class="label">${Y.label} <span style="color:var(--muted)">· ${Y.model||"?"}</span></span>
        <span class="uid">${Y.uid}</span>
      </div>`)}`}renderRecent(){return q`
      <section class="section">
        <h3>Recent (24h)</h3>
        ${this.recentError?q`<div class="err">⚠ ${this.recentError}</div>`:J}
        ${this.recent.length===0?q`<div class="panel"><div class="empty">No fault events in the last 24h.</div></div>`:q`<div class="panel"><events-table .events=${this.recent}></events-table></div>`}
      </section>
    `}render(){return q`${this.renderLive()}${this.renderRecent()}`}}customElements.define("alarms-view",j9);class $9 extends ${static properties={events:{state:!0},error:{state:!0},loading:{state:!0}};timer=null;constructor(){super();this.events=[],this.error="",this.loading=!1}static styles=L`
    :host { display: block; }
    .bar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
    .count { color: var(--muted); font-size: 13px; }
    button {
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 8px;
      padding: 6px 12px;
      font-size: 13px;
      cursor: pointer;
    }
    button:hover { color: var(--text); border-color: var(--muted); }
    .err { color: var(--err); font-size: 13px; margin-bottom: 12px; }
    .panel { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; overflow: hidden; }
  `;connectedCallback(){super.connectedCallback(),this.load(),this.timer=setInterval(()=>void this.load(),15000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async load(){this.loading=!0;try{let Q=await D.events({limit:200});this.events=Q.events??[],this.error=Q.error??""}catch(Q){this.error=Q.message}finally{this.loading=!1}}render(){return q`
      <div class="bar">
        <span class="count">${this.events.length} event(s)${this.loading?" · refreshing…":""}</span>
        <button @click=${()=>void this.load()}>Refresh</button>
      </div>
      ${this.error?q`<div class="err">⚠ ${this.error}</div>`:J}
      <div class="panel"><events-table .events=${this.events}></events-table></div>
    `}}customElements.define("events-view",$9);var A9;function O(Q,Y,K){function X(H,W){if(!H._zod)Object.defineProperty(H,"_zod",{value:{def:W,constr:B,traits:new Set},enumerable:!1});if(H._zod.traits.has(Q))return;H._zod.traits.add(Q),Y(H,W);let U=B.prototype,j=Object.keys(U);for(let A=0;A<j.length;A++){let F=j[A];if(!(F in H))H[F]=U[F].bind(H)}}let G=K?.Parent??Object;class z extends G{}Object.defineProperty(z,"name",{value:Q});function B(H){var W;let U=K?.Parent?new z:this;X(U,H),(W=U._zod).deferred??(W.deferred=[]);for(let j of U._zod.deferred)j();return U}return Object.defineProperty(B,"init",{value:X}),Object.defineProperty(B,Symbol.hasInstance,{value:(H)=>{if(K?.Parent&&H instanceof K.Parent)return!0;return H?._zod?.traits?.has(Q)}}),Object.defineProperty(B,"name",{value:Q}),B}var J6=Symbol("zod_brand");class u extends Error{constructor(){super("Encountered Promise during synchronous parse. Use .parseAsync() instead.")}}class J3 extends Error{constructor(Q){super(`Encountered unidirectional transform during encode: ${Q}`);this.name="ZodEncodeError"}}(A9=globalThis).__zod_globalConfig??(A9.__zod_globalConfig={});var B3=globalThis.__zod_globalConfig;function G4(Q){if(Q)Object.assign(B3,Q);return B3}function L9(Q,Y){if(typeof Y==="bigint")return Y.toString();return Y}function F9(Q){return{get value(){{let K=Q();return Object.defineProperty(this,"value",{value:K}),K}throw Error("cached value already set")}}}function I9(Q){return Q===null||Q===void 0}function V9(Q){let Y=Q.startsWith("^")?1:0,K=Q.endsWith("$")?Q.length-1:Q.length;return Q.slice(Y,K)}var D9=Symbol("evaluating");function x(Q,Y,K){let X=void 0;Object.defineProperty(Q,Y,{get(){if(X===D9)return;if(X===void 0)X=D9,X=K();return X},set(G){Object.defineProperty(Q,Y,{value:G})},configurable:!0})}var W3="captureStackTrace"in Error?Error.captureStackTrace:(...Q)=>{};function N9(Q){return typeof Q==="object"&&Q!==null&&!Array.isArray(Q)}function U3(Q,Y,K){let X=new Q._zod.constr(Y??Q._zod.def);if(!Y||K?.parent)X._zod.parent=Q;return X}function f(Q){let Y=Q;if(!Y)return{};if(typeof Y==="string")return{error:()=>Y};if(Y?.message!==void 0){if(Y?.error!==void 0)throw Error("Cannot specify both `message` and `error` params");Y.error=Y.message}if(delete Y.message,typeof Y.error==="string")return{...Y,error:()=>Y.error};return Y}function O9(Q){return Object.keys(Q).filter((Y)=>{return Q[Y]._zod.optin==="optional"&&Q[Y]._zod.optout==="optional"})}var eQ={safeint:[Number.MIN_SAFE_INTEGER,Number.MAX_SAFE_INTEGER],int32:[-2147483648,2147483647],uint32:[0,4294967295],float32:[-340282346638528860000000000000000000000,340282346638528860000000000000000000000],float64:[-Number.MAX_VALUE,Number.MAX_VALUE]};function O4(Q,Y=0){if(Q.aborted===!0)return!0;for(let K=Y;K<Q.issues.length;K++)if(Q.issues[K]?.continue!==!0)return!0;return!1}function P9(Q,Y=0){if(Q.aborted===!0)return!0;for(let K=Y;K<Q.issues.length;K++)if(Q.issues[K]?.continue===!1)return!0;return!1}function v4(Q,Y){return Y.map((K)=>{var X;return(X=K).path??(X.path=[]),K.path.unshift(Q),K})}function g4(Q){return typeof Q==="string"?Q:Q?.message}function q4(Q,Y,K){let X=Q.message?Q.message:g4(Q.inst?._zod.def?.error?.(Q))??g4(Y?.error?.(Q))??g4(K.customError?.(Q))??g4(K.localeError?.(Q))??"Invalid input",{inst:G,continue:z,input:B,...H}=Q;if(H.path??(H.path=[]),H.message=X,Y?.reportInput)H.input=B;return H}function E9(Q){if(Array.isArray(Q))return"array";if(typeof Q==="string")return"string";return"unknown"}var M9=(Q,Y)=>{Q.name="$ZodError",Object.defineProperty(Q,"_zod",{value:Q._zod,enumerable:!1}),Object.defineProperty(Q,"issues",{value:Y,enumerable:!1}),Q.message=JSON.stringify(Y,L9,2),Object.defineProperty(Q,"toString",{value:()=>Q.message,enumerable:!1})},Z9=O("$ZodError",M9),P4=O("$ZodError",M9,{Parent:Error});var Y8=(Q)=>(Y,K,X,G)=>{let z=X?{...X,async:!1}:{async:!1},B=Y._zod.run({value:K,issues:[]},z);if(B instanceof Promise)throw new u;if(B.issues.length){let H=new(G?.Err??Q)(B.issues.map((W)=>q4(W,z,G4())));throw W3(H,G?.callee),H}return B.value},h4=Y8(P4),K8=(Q)=>async(Y,K,X,G)=>{let z=X?{...X,async:!0}:{async:!0},B=Y._zod.run({value:K,issues:[]},z);if(B instanceof Promise)B=await B;if(B.issues.length){let H=new(G?.Err??Q)(B.issues.map((W)=>q4(W,z,G4())));throw W3(H,G?.callee),H}return B.value},c4=K8(P4),X8=(Q)=>(Y,K,X)=>{let G=X?{...X,async:!1}:{async:!1},z=Y._zod.run({value:K,issues:[]},G);if(z instanceof Promise)throw new u;return z.issues.length?{success:!1,error:new(Q??Z9)(z.issues.map((B)=>q4(B,G,G4())))}:{success:!0,data:z.value}},i=X8(P4),G8=(Q)=>async(Y,K,X)=>{let G=X?{...X,async:!0}:{async:!0},z=Y._zod.run({value:K,issues:[]},G);if(z instanceof Promise)z=await z;return z.issues.length?{success:!1,error:new Q(z.issues.map((B)=>q4(B,G,G4())))}:{success:!0,data:z.value}},E4=G8(P4);var q8="(?:(?:\\d\\d[2468][048]|\\d\\d[13579][26]|\\d\\d0[48]|[02468][048]00|[13579][26]00)-02-29|\\d{4}-(?:(?:0[13578]|1[02])-(?:0[1-9]|[12]\\d|3[01])|(?:0[469]|11)-(?:0[1-9]|[12]\\d|30)|(?:02)-(?:0[1-9]|1\\d|2[0-8])))",z8=new RegExp(`^${q8}$`);var R9=(Q)=>{let Y=Q?`[\\s\\S]{${Q?.minimum??0},${Q?.maximum??""}}`:"[\\s\\S]*";return new RegExp(`^${Y}$`)};var _9=/^-?\d+(?:\.\d+)?$/;var B4=O("$ZodCheck",(Q,Y)=>{var K;Q._zod??(Q._zod={}),Q._zod.def=Y,(K=Q._zod).onattach??(K.onattach=[])});var T9=O("$ZodCheckMinLength",(Q,Y)=>{var K;B4.init(Q,Y),(K=Q._zod.def).when??(K.when=(X)=>{let G=X.value;return!I9(G)&&G.length!==void 0}),Q._zod.onattach.push((X)=>{let G=X._zod.bag.minimum??Number.NEGATIVE_INFINITY;if(Y.minimum>G)X._zod.bag.minimum=Y.minimum}),Q._zod.check=(X)=>{let G=X.value;if(G.length>=Y.minimum)return;let B=E9(G);X.issues.push({origin:B,code:"too_small",minimum:Y.minimum,inclusive:!0,input:G,inst:Q,continue:!Y.abort})}});var J8=O("$ZodCheckStringFormat",(Q,Y)=>{var K,X;if(B4.init(Q,Y),Q._zod.onattach.push((G)=>{let z=G._zod.bag;if(z.format=Y.format,Y.pattern)z.patterns??(z.patterns=new Set),z.patterns.add(Y.pattern)}),Y.pattern)(K=Q._zod).check??(K.check=(G)=>{if(Y.pattern.lastIndex=0,Y.pattern.test(G.value))return;G.issues.push({origin:"string",code:"invalid_format",format:Y.format,input:G.value,...Y.pattern?{pattern:Y.pattern.toString()}:{},inst:Q,continue:!Y.abort})});else(X=Q._zod).check??(X.check=()=>{})}),S9=O("$ZodCheckRegex",(Q,Y)=>{J8.init(Q,Y),Q._zod.check=(K)=>{if(Y.pattern.lastIndex=0,Y.pattern.test(K.value))return;K.issues.push({origin:"string",code:"invalid_format",format:"regex",input:K.value,pattern:Y.pattern.toString(),inst:Q,continue:!Y.abort})}});var b9=O("$ZodCheckOverwrite",(Q,Y)=>{B4.init(Q,Y),Q._zod.check=(K)=>{K.value=Y.tx(K.value)}});var C9={major:4,minor:4,patch:3};var o=O("$ZodType",(Q,Y)=>{var K;Q??(Q={}),Q._zod.def=Y,Q._zod.bag=Q._zod.bag||{},Q._zod.version=C9;let X=[...Q._zod.def.checks??[]];if(Q._zod.traits.has("$ZodCheck"))X.unshift(Q);for(let G of X)for(let z of G._zod.onattach)z(Q);if(X.length===0)(K=Q._zod).deferred??(K.deferred=[]),Q._zod.deferred?.push(()=>{Q._zod.run=Q._zod.parse});else{let G=(B,H,W)=>{let U=O4(B),j;for(let A of H){if(A._zod.def.when){if(P9(B))continue;if(!A._zod.def.when(B))continue}else if(U)continue;let F=B.issues.length,I=A._zod.check(B);if(I instanceof Promise&&W?.async===!1)throw new u;if(j||I instanceof Promise)j=(j??Promise.resolve()).then(async()=>{if(await I,B.issues.length===F)return;if(!U)U=O4(B,F)});else{if(B.issues.length===F)continue;if(!U)U=O4(B,F)}}if(j)return j.then(()=>{return B});return B},z=(B,H,W)=>{if(O4(B))return B.aborted=!0,B;let U=G(H,X,W);if(U instanceof Promise){if(W.async===!1)throw new u;return U.then((j)=>Q._zod.parse(j,W))}return Q._zod.parse(U,W)};Q._zod.run=(B,H)=>{if(H.skipChecks)return Q._zod.parse(B,H);if(H.direction==="backward"){let U=Q._zod.parse({value:B.value,issues:[]},{...H,skipChecks:!0});if(U instanceof Promise)return U.then((j)=>{return z(j,B,H)});return z(U,B,H)}let W=Q._zod.parse(B,H);if(W instanceof Promise){if(H.async===!1)throw new u;return W.then((U)=>G(U,X,H))}return G(W,X,H)}}x(Q,"~standard",()=>({validate:(G)=>{try{let z=i(Q,G);return z.success?{value:z.data}:{issues:z.error?.issues}}catch(z){return E4(Q,G).then((B)=>B.success?{value:B.data}:{issues:B.error?.issues})}},vendor:"zod",version:1}))}),g9=O("$ZodString",(Q,Y)=>{o.init(Q,Y),Q._zod.pattern=[...Q?._zod.bag?.patterns??[]].pop()??R9(Q._zod.bag),Q._zod.parse=(K,X)=>{if(Y.coerce)try{K.value=String(K.value)}catch(G){}if(typeof K.value==="string")return K;return K.issues.push({expected:"string",code:"invalid_type",input:K.value,inst:Q}),K}});var v9=O("$ZodNumber",(Q,Y)=>{o.init(Q,Y),Q._zod.pattern=Q._zod.bag.pattern??_9,Q._zod.parse=(K,X)=>{if(Y.coerce)try{K.value=Number(K.value)}catch(B){}let G=K.value;if(typeof G==="number"&&!Number.isNaN(G)&&Number.isFinite(G))return K;let z=typeof G==="number"?Number.isNaN(G)?"NaN":!Number.isFinite(G)?"Infinity":void 0:void 0;return K.issues.push({expected:"number",code:"invalid_type",input:G,inst:Q,...z?{received:z}:{}}),K}});function k9(Q,Y,K){if(Q.issues.length)Y.issues.push(...v4(K,Q.issues));Y.value[K]=Q.value}var h9=O("$ZodArray",(Q,Y)=>{o.init(Q,Y),Q._zod.parse=(K,X)=>{let G=K.value;if(!Array.isArray(G))return K.issues.push({expected:"array",code:"invalid_type",input:G,inst:Q}),K;K.value=Array(G.length);let z=[];for(let B=0;B<G.length;B++){let H=G[B],W=Y.element._zod.run({value:H,issues:[]},X);if(W instanceof Promise)z.push(W.then((U)=>k9(U,K,B)));else k9(W,K,B)}if(z.length)return Promise.all(z).then(()=>K);return K}});function m4(Q,Y,K,X,G,z){let B=K in X;if(Q.issues.length){if(G&&z&&!B)return;Y.issues.push(...v4(K,Q.issues))}if(!B&&!G){if(!Q.issues.length)Y.issues.push({code:"invalid_type",expected:"nonoptional",input:void 0,path:[K]});return}if(Q.value===void 0){if(B)Y.value[K]=void 0}else Y.value[K]=Q.value}function U8(Q){let Y=Object.keys(Q.shape);for(let X of Y)if(!Q.shape?.[X]?._zod?.traits?.has("$ZodType"))throw Error(`Invalid element at key "${X}": expected a Zod schema`);let K=O9(Q.shape);return{...Q,keys:Y,keySet:new Set(Y),numKeys:Y.length,optionalKeys:new Set(K)}}function j8(Q,Y,K,X,G,z){let B=[],H=G.keySet,W=G.catchall._zod,U=W.def.type,j=W.optin==="optional",A=W.optout==="optional";for(let F in Y){if(F==="__proto__")continue;if(H.has(F))continue;if(U==="never"){B.push(F);continue}let I=W.run({value:Y[F],issues:[]},X);if(I instanceof Promise)Q.push(I.then((P)=>m4(P,K,F,Y,j,A)));else m4(I,K,F,Y,j,A)}if(B.length)K.issues.push({code:"unrecognized_keys",keys:B,input:Y,inst:z});if(!Q.length)return K;return Promise.all(Q).then(()=>{return K})}var c9=O("$ZodObject",(Q,Y)=>{if(o.init(Q,Y),!Object.getOwnPropertyDescriptor(Y,"shape")?.get){let H=Y.shape;Object.defineProperty(Y,"shape",{get:()=>{let W={...H};return Object.defineProperty(Y,"shape",{value:W}),W}})}let X=F9(()=>U8(Y));x(Q._zod,"propValues",()=>{let H=Y.shape,W={};for(let U in H){let j=H[U]._zod;if(j.values){W[U]??(W[U]=new Set);for(let A of j.values)W[U].add(A)}}return W});let G=N9,z=Y.catchall,B;Q._zod.parse=(H,W)=>{B??(B=X.value);let U=H.value;if(!G(U))return H.issues.push({expected:"object",code:"invalid_type",input:U,inst:Q}),H;H.value={};let j=[],A=B.shape;for(let F of B.keys){let I=A[F],P=I._zod.optin==="optional",N=I._zod.optout==="optional",c=I._zod.run({value:U[F],issues:[]},W);if(c instanceof Promise)j.push(c.then((g)=>m4(g,H,F,U,P,N)));else m4(c,H,F,U,P,N)}if(!z)return j.length?Promise.all(j).then(()=>H):H;return j8(j,U,H,W,X.value,Q)}});var y9=O("$ZodTransform",(Q,Y)=>{o.init(Q,Y),Q._zod.optin="optional",Q._zod.parse=(K,X)=>{if(X.direction==="backward")throw new J3(Q.constructor.name);let G=Y.transform(K.value,K);if(X.async)return(G instanceof Promise?G:Promise.resolve(G)).then((B)=>{return K.value=B,K.fallback=!0,K});if(G instanceof Promise)throw new u;return K.value=G,K.fallback=!0,K}});function x9(Q,Y){if(Y===void 0&&(Q.issues.length||Q.fallback))return{issues:[],value:void 0};return Q}var m9=O("$ZodOptional",(Q,Y)=>{o.init(Q,Y),Q._zod.optin="optional",Q._zod.optout="optional",x(Q._zod,"values",()=>{return Y.innerType._zod.values?new Set([...Y.innerType._zod.values,void 0]):void 0}),x(Q._zod,"pattern",()=>{let K=Y.innerType._zod.pattern;return K?new RegExp(`^(${V9(K.source)})?$`):void 0}),Q._zod.parse=(K,X)=>{if(Y.innerType._zod.optin==="optional"){let G=K.value,z=Y.innerType._zod.run(K,X);if(z instanceof Promise)return z.then((B)=>x9(B,G));return x9(z,G)}if(K.value===void 0)return K;return Y.innerType._zod.run(K,X)}});var u9=O("$ZodPipe",(Q,Y)=>{o.init(Q,Y),x(Q._zod,"values",()=>Y.in._zod.values),x(Q._zod,"optin",()=>Y.in._zod.optin),x(Q._zod,"optout",()=>Y.out._zod.optout),x(Q._zod,"propValues",()=>Y.in._zod.propValues),Q._zod.parse=(K,X)=>{if(X.direction==="backward"){let z=Y.out._zod.run(K,X);if(z instanceof Promise)return z.then((B)=>y4(B,Y.in,X));return y4(z,Y.in,X)}let G=Y.in._zod.run(K,X);if(G instanceof Promise)return G.then((z)=>y4(z,Y.out,X));return y4(G,Y.out,X)}});function y4(Q,Y,K){if(Q.issues.length)return Q.aborted=!0,Q;return Y._zod.run({value:Q.value,issues:Q.issues,fallback:Q.fallback},K)}function f9(Q,Y){return new Q({type:"string",...f(Y)})}function o9(Q,Y){return new Q({type:"number",checks:[],...f(Y)})}function M4(Q,Y){return new T9({check:"min_length",...f(Y),minimum:Q})}function u4(Q,Y){return new S9({check:"string_format",format:"regex",...f(Y),pattern:Q})}function r9(Q){return new b9({check:"overwrite",tx:Q})}function j3(){return r9((Q)=>Q.trim())}var s=O("ZodMiniType",(Q,Y)=>{if(!Q._zod)throw Error("Uninitialized schema in ZodMiniType.");o.init(Q,Y),Q.def=Y,Q.type=Y.type,Q.parse=(K,X)=>h4(Q,K,X,{callee:Q.parse}),Q.safeParse=(K,X)=>i(Q,K,X),Q.parseAsync=async(K,X)=>c4(Q,K,X,{callee:Q.parseAsync}),Q.safeParseAsync=async(K,X)=>E4(Q,K,X),Q.check=(...K)=>{return Q.clone({...Y,checks:[...Y.checks??[],...K.map((X)=>typeof X==="function"?{_zod:{check:X,def:{check:"custom"},onattach:[]}}:X)]},{parent:!0})},Q.with=Q.check,Q.clone=(K,X)=>U3(Q,K,X),Q.brand=()=>Q,Q.register=(K,X)=>{return K.add(Q,X),Q},Q.apply=(K)=>K(Q)}),D8=O("ZodMiniString",(Q,Y)=>{g9.init(Q,Y),s.init(Q,Y)});function J4(Q){return f9(D8,Q)}var L8=O("ZodMiniNumber",(Q,Y)=>{v9.init(Q,Y),s.init(Q,Y)});function p9(Q){return o9(L8,Q)}var F8=O("ZodMiniArray",(Q,Y)=>{h9.init(Q,Y),s.init(Q,Y)});function $3(Q,Y){return new F8({type:"array",element:Q,...f(Y)})}var I8=O("ZodMiniObject",(Q,Y)=>{c9.init(Q,Y),s.init(Q,Y),x(Q,"shape",()=>Y.shape)});function A3(Q,Y){let K={type:"object",shape:Q??{},...f(Y)};return new I8(K)}var V8=O("ZodMiniTransform",(Q,Y)=>{y9.init(Q,Y),s.init(Q,Y)});function l9(Q){return new V8({type:"transform",transform:Q})}var N8=O("ZodMiniOptional",(Q,Y)=>{m9.init(Q,Y),s.init(Q,Y)});function D3(Q){return new N8({type:"optional",innerType:Q})}var O8=O("ZodMiniPipe",(Q,Y)=>{u9.init(Q,Y),s.init(Q,Y)});function d9(Q,Y){return new O8({type:"pipe",in:Q,out:Y})}function n9(Q,Y){let K=new B4({check:"custom",...f(Y)});return K._zod.check=Q,K}var i9="invdriver.gridprofile/v1",M8=J4().check(u4(/^[A-Z]{2}$/,"must match ^[A-Z]{2}$")),Z8=J4().check(u4(/^[0-9A-Fa-f]{12}$/,"must be 12 hex characters")),R8=d9(l9((Q)=>{if(Q&&typeof Q==="object"){let Y=Q,K="aps_code"in Y||"value"in Y,X="apply"in Y||"native"in Y;if(K)return{aps_code:Y.aps_code,value:Y.value,unit:Y.unit};if(X){let G=Y.apply??{},z=Y.native??{};return{aps_code:G.aps_code,value:z.value,unit:z.unit}}}return Q}),A3({aps_code:M8,value:p9(),unit:D3(J4())})),_8=A3({schema:D3(J4()),id:J4().check(j3(),M4(1,"must be a non-empty string")),uids:$3(Z8).check(M4(1,"must contain at least one inverter UID")),points:$3(R8).check(M4(1,"must contain at least one parameter override"))}),w8=_8.check(n9((Q)=>{let Y=Q.value,K=new Map;for(let G=0;G<Y.points.length;G++){let z=Y.points[G].aps_code,B=K.get(z);if(B!==void 0)Q.issues.push({code:"custom",path:["points",G,"aps_code"],message:`duplicate aps_code "${z}" (also at points[${B}])`,input:Y});else K.set(z,G)}let X=new Map;for(let G=0;G<Y.uids.length;G++){let z=Y.uids[G].toLowerCase(),B=X.get(z);if(B!==void 0)Q.issues.push({code:"custom",path:["uids",G],message:`duplicate uid "${Y.uids[G]}" (also at uids[${B}])`,input:Y});else X.set(z,G)}}));function T8(Q){let Y="";for(let K of Q)if(typeof K==="number")Y+=`[${K}]`;else Y+=Y?`.${String(K)}`:String(K);return Y||"(root)"}function s9(Q){let Y=i(w8,Q);if(!Y.success)return{ok:!1,errors:Y.error.issues.map((B)=>`${T8(B.path)}: ${B.message}`)};let K=[];if(Y.data.schema!==void 0&&Y.data.schema!==i9)K.push(`schema tag "${Y.data.schema}" does not match expected "${i9}"`);let X=Y.data.points.map((z)=>{let B={aps_code:z.aps_code,value:z.value};if(z.unit!==void 0&&z.unit!=="")B.unit=z.unit;return B});return{ok:!0,profile:{id:Y.data.id.trim(),uids:Y.data.uids,points:X},warnings:K}}class t9 extends ${static properties={profiles:{attribute:!1},activeBase:{attribute:!1},reconcilerReady:{attribute:!1},busy:{attribute:!1},selected:{state:!0}};constructor(){super();this.profiles=[],this.activeBase="",this.reconcilerReady=!0,this.busy=!1,this.selected=""}static styles=L`
    :host { display: block; }
    .grid { display: grid; gap: 16px; max-width: 460px; }
    .active { font-size: 14px; color: var(--text); }
    .active .muted { color: var(--muted); }
    .active .none { color: var(--muted); font-style: italic; }
    label { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: var(--muted); min-width: 0; }
    select {
      width: 100%;
      max-width: 100%;
      box-sizing: border-box;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      color: var(--text);
      border-radius: 8px;
      padding: 9px 11px;
      font-size: 14px;
      font-family: inherit;
    }
    select:focus { outline: none; border-color: var(--accent); }
    .actions { display: flex; align-items: center; gap: 12px; margin-top: 4px; }
    button.apply {
      background: var(--accent);
      border: none;
      color: #04121a;
      border-radius: 8px;
      padding: 9px 18px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
    }
    button.apply:hover:not(:disabled) { filter: brightness(1.08); }
    button.apply:disabled { opacity: 0.45; cursor: not-allowed; }
    .hint { font-size: 12px; color: var(--muted); }
  `;onChange=(Q)=>{this.selected=Q.target.value};apply=()=>{let Q=this.effectiveSelected();if(!Q||Q===this.activeBase)return;this.dispatchEvent(new CustomEvent("apply",{detail:Q,bubbles:!0,composed:!0}))};effectiveSelected(){return this.selected||this.activeBase}labelFor(Q){let Y=[`${Q.vnom_v} V`];if(Q.source_ref)Y.push(Q.source_ref);return Y.push(`${Q.point_count} pts`),`${Q.id} — ${Y.join(" · ")}`}render(){let Q=this.effectiveSelected(),Y=this.profiles.find((X)=>X.id===this.activeBase),K=!this.busy&&this.reconcilerReady&&Q!==""&&Q!==this.activeBase;return q`
      <div class="grid">
        <div class="active">
          <span class="muted">Active profile:</span>
          ${this.activeBase?q` <strong>${this.activeBase}</strong>${Y?q` <span class="muted">(${Y.vnom_v} V · ${Y.point_count} pts)</span>`:J}`:q` <span class="none">none selected</span>`}
        </div>

        <label>
          Base profile
          <select id="profile" .value=${Q} @change=${this.onChange} ?disabled=${this.busy}>
            ${this.activeBase?J:q`<option value="" disabled selected>Select a profile…</option>`}
            ${this.profiles.map((X)=>q`<option value=${X.id} ?selected=${X.id===Q}>${this.labelFor(X)}</option>`)}
          </select>
        </label>

        <div class="actions">
          <button class="apply" @click=${this.apply} ?disabled=${!K}>
            ${this.busy?"Applying…":"Apply"}
          </button>
          ${!this.reconcilerReady?q`<span class="hint">reconciler not ready</span>`:Q&&Q!==this.activeBase?q`<span class="hint">applies to all inverters</span>`:J}
        </div>
      </div>
    `}}customElements.define("grid-profile-form",t9);var a9={AC:{label:"Undervoltage trip — stage 2",desc:"Disconnect when AC voltage drops to this lower-stage level."},AQ:{label:"Undervoltage trip — deep",desc:"Disconnect quickly when voltage falls this far below nominal."},AH:{label:"Undervoltage trip — fast",desc:"Fast disconnect on a severe undervoltage."},AD:{label:"Overvoltage trip — slow",desc:"Disconnect when AC voltage rises above this (slower stage)."},AY:{label:"Overvoltage trip — slow (stage 2)",desc:"Second slower overvoltage disconnect threshold."},AB:{label:"10-minute mean overvoltage",desc:"Trips if the 10-minute average voltage exceeds this (EN 50549 sustained-overvoltage limit)."},AI:{label:"Overvoltage trip — fast",desc:"Fast disconnect on a severe overvoltage."},AE:{label:"Underfrequency trip — slow",desc:"Disconnect when grid frequency falls below this (slower stage)."},AJ:{label:"Underfrequency trip — fast",desc:"Fast disconnect on a severe underfrequency."},AF:{label:"Overfrequency trip — slow",desc:"Disconnect when grid frequency rises above this (slower stage)."},AK:{label:"Overfrequency trip — fast",desc:"Fast disconnect on a severe overfrequency."},BB:{label:"Undervoltage 1 — clearance time",desc:"How long the undervoltage condition must persist before tripping."},BD:{label:"Undervoltage 2 — clearance time",desc:"Clearance delay for the second undervoltage stage."},BC:{label:"Overvoltage 1 — clearance time",desc:"How long the overvoltage condition must persist before tripping."},BE:{label:"Overvoltage 2 — clearance time",desc:"Clearance delay for the second overvoltage stage."},BH:{label:"Underfrequency 1 — clearance time",desc:"Clearance delay for the first underfrequency stage."},BJ:{label:"Underfrequency 2 — clearance time",desc:"Clearance delay for the second underfrequency stage."},BI:{label:"Overfrequency 1 — clearance time",desc:"Clearance delay for the first overfrequency stage."},BK:{label:"Overfrequency 2 — clearance time",desc:"Clearance delay for the second overfrequency stage."},BN:{label:"Enter-service voltage — lower",desc:"Voltage must be above this before the inverter reconnects."},BO:{label:"Enter-service voltage — upper",desc:"Voltage must be below this before the inverter reconnects."},BP:{label:"Enter-service frequency — lower",desc:"Frequency must be above this before the inverter reconnects."},BQ:{label:"Enter-service frequency — upper",desc:"Frequency must be below this before the inverter reconnects."},AG:{label:"Grid-recovery delay",desc:"Wait time after the grid is healthy before reconnecting."},AS:{label:"Power ramp time",desc:"Time taken to ramp output back up after reconnecting."},CV:{label:"Curtailment enable (droop)",desc:"Enables the over-frequency droop power reduction (0 = off, 1 = on)."},CA:{label:"Curtailment start (droop deadband)",desc:"Over-frequency droop: power reduction begins at this frequency (deadband end)."},DD:{label:"Curtailment slope (droop)",desc:"Over-frequency droop gradient: % of rated power reduced per Hz above the start."},CG:{label:"Curtailment response time (droop)",desc:"Filter/response time of the droop control loop."},DH:{label:"Under-freq curve — low",desc:"Legacy frequency-Watt curve: lower frequency point of the under-frequency response."},DI:{label:"Under-freq curve — high",desc:"Legacy frequency-Watt curve: upper frequency point of the under-frequency response."},CB:{label:"Over-freq curve — start",desc:"Legacy frequency-Watt curve: over-frequency power reduction begins at this frequency."},CC:{label:"Over-freq curve — end",desc:"Legacy frequency-Watt curve: over-frequency reduction reaches its limit at this frequency."}},e9={DERFreqDroop:{label:"Frequency-Watt droop",tip:"Linearly reduces active power as frequency rises above a deadband — over-frequency curtailment (SunSpec DERFreqDroop, model 711)."},CrvSet:{label:"Frequency-Watt curve",tip:"Legacy point-based power-versus-frequency response curve (model 134)."},MustTrip:{label:"Trip thresholds",tip:"Voltage and frequency limits that disconnect the inverter from the grid when crossed (protection trips)."},DEREnterService:{label:"Enter service",tip:"The voltage/frequency window and timing the inverter must satisfy before (re)connecting after a trip."}},L3=["DERFreqDroop","CrvSet","MustTrip","DEREnterService"],QQ=new Set(["MustTrip","DEREnterService"]);function S8(Q,Y){if(!Q)return Y;return Q.replace(/_/g," ").replace(/\b\w/g,(K)=>K.toUpperCase())}function YQ(Q,Y){return a9[Q]?.label??S8(Y??"",Q)}function KQ(Q){return a9[Q]?.desc??""}function F3(Q,Y){let K=[];for(let X of Q){let G=Y(X.left),z=Y(X.right);if(G!==void 0&&z!==void 0&&!(G<z))K.push(X.message)}return K}class XQ extends ${static properties={deadband:{type:Number},slope:{type:Number},trip:{type:Number},nominal:{type:Number}};constructor(){super();this.nominal=50}static styles=L`
    :host { display: block; }
    svg { width: 100%; height: auto; }
    .frame { stroke: var(--border); fill: none; }
    .grid { stroke: color-mix(in srgb, var(--border) 60%, transparent); }
    .curve { stroke: var(--accent); stroke-width: 2; fill: none; }
    .dead { stroke: var(--muted); stroke-dasharray: 3 3; }
    .trip { stroke: var(--err); stroke-width: 1.5; }
    text { fill: var(--muted); font-size: 9px; }
    .lbl { fill: var(--text); }
    .empty { color: var(--muted); font-size: 12px; padding: 8px 0; }
  `;render(){let Q=this.deadband,Y=this.slope,K=this.trip,X=this.nominal;if(Q===void 0||Y===void 0||Y<=0)return q`<div class="empty">Set the curtailment start frequency and slope to preview the curve.</div>`;let G=Q+100/Y,z=X-0.3,B=Math.max(K??0,G,Q+1.5,X+1.5)+0.2,H=480,W=170,U=36,j=12,A=10,F=24,I=(E)=>U+(E-z)/(B-z)*(H-U-j),P=(E)=>A+(100-E)/100*(W-A-F),N=Math.min(G,B),c=Math.max(0,100-Y*(N-Q)),g=[[z,100],[Q,100],[N,c],...G<B?[[B,0]]:[]].map(([E,IQ])=>`${I(E).toFixed(1)},${P(IQ).toFixed(1)}`).join(" "),f4=[];for(let E=Math.ceil(z*2)/2;E<=B;E+=0.5)f4.push(E);return q`
      <svg viewBox="0 0 ${H} ${W}" role="img" aria-label="Frequency-Watt curtailment curve">
        ${[0,50,100].map((E)=>S`<line class="grid" x1=${U} y1=${P(E)} x2=${H-j} y2=${P(E)} />
            <text x=${U-4} y=${P(E)+3} text-anchor="end">${E}%</text>`)}
        ${f4.map((E)=>S`<text x=${I(E)} y=${W-F+12} text-anchor="middle">${E.toFixed(1)}</text>`)}
        <line class="frame" x1=${U} y1=${A} x2=${U} y2=${W-F} />
        <line class="frame" x1=${U} y1=${W-F} x2=${H-j} y2=${W-F} />
        <line class="dead" x1=${I(Q)} y1=${A} x2=${I(Q)} y2=${W-F} />
        <text class="lbl" x=${I(Q)} y=${A+8} text-anchor="middle">start ${Z(Q)} Hz</text>
        ${G<=B?S`<line class="dead" x1=${I(G)} y1=${A} x2=${I(G)} y2=${W-F} />
              <text class="lbl" x=${I(G)} y=${A+8} text-anchor="middle">0% at ${Z(G)} Hz</text>`:J}
        ${K!==void 0&&K>=z&&K<=B?S`<line class="trip" x1=${I(K)} y1=${A} x2=${I(K)} y2=${W-F} />
              <text x=${I(K)} y=${W-F-4} text-anchor="middle" fill="var(--err)">trip ${Z(K)} Hz</text>`:J}
        <polyline class="curve" points=${g} />
        <text x=${H/2} y=${W-2} text-anchor="middle">Power vs frequency · slope ${Z(Y)} %Pref/Hz</text>
      </svg>
    `}}customElements.define("freq-watt-chart",XQ);class GQ extends ${static properties={unit:{type:String},nominal:{type:Number},markers:{attribute:!1}};constructor(){super();this.unit="",this.markers=[]}static styles=L`
    :host { display: block; }
    svg { width: 100%; height: auto; }
    .axis { stroke: var(--border); }
    .band { fill: color-mix(in srgb, var(--ok) 16%, transparent); }
    .under { stroke: var(--accent); }
    .over { stroke: var(--err); }
    .curve { stroke: var(--muted); stroke-dasharray: 2 2; }
    .nom { stroke: var(--ok); }
    text { font-size: 9px; fill: var(--muted); }
    .empty { color: var(--muted); font-size: 12px; padding: 6px 0; }
  `;render(){let Q=(this.markers??[]).filter((N)=>Number.isFinite(N.value));if(!Q.length)return q`<div class="empty">No thresholds set.</div>`;let Y=Q.map((N)=>N.value).concat(this.nominal!==void 0?[this.nominal]:[]),K=Math.min(...Y),X=Math.max(...Y),G=(X-K)*0.14||1;K-=G,X+=G;let z=480,B=70,H=10,W=10,U=34,j=(N)=>H+(N-K)/(X-K)*(z-H-W),A=Q.filter((N)=>N.kind==="under").map((N)=>N.value),F=Q.filter((N)=>N.kind==="over").map((N)=>N.value),I=A.length?Math.max(...A):K,P=F.length?Math.min(...F):X;return q`
      <svg viewBox="0 0 ${z} ${B}" role="img" aria-label="Trip thresholds">
        ${P>I?S`<rect class="band" x=${j(I)} y=${U-8} width=${j(P)-j(I)} height=16 />`:J}
        <line class="axis" x1=${H} y1=${U} x2=${z-W} y2=${U} />
        ${this.nominal!==void 0?S`<line class="nom" x1=${j(this.nominal)} y1=${U-9} x2=${j(this.nominal)} y2=${U+9} />
              <text x=${j(this.nominal)} y=${U+20} text-anchor="middle" fill="var(--ok)">${Z(this.nominal)} ${this.unit}</text>`:J}
        ${Q.map((N,c)=>{let g=N.kind,E=c%2===0?U-12:U+22;return S`<line class=${g} x1=${j(N.value)} y1=${U-7} x2=${j(N.value)} y2=${U+7} />
            <text x=${j(N.value)} y=${E} text-anchor="middle">${N.label} ${Z(N.value)}</text>`})}
      </svg>
    `}}customElements.define("trip-line",GQ);class qQ extends ${static properties={params:{attribute:!1},inverters:{attribute:!1},defaults:{attribute:!1},rules:{attribute:!1},profile:{attribute:!1},names:{attribute:!1},busy:{attribute:!1},editing:{attribute:!1},name:{state:!0},selectedUids:{state:!0},values:{state:!0},localError:{state:!0}};constructor(){super();this.params=[],this.inverters=[],this.defaults={},this.rules=[],this.profile=null,this.names={},this.busy=!1,this.editing=!1,this.name="",this.selectedUids=[],this.values={},this.localError=""}static styles=L`
    :host { display: block; }
    .grid { display: grid; gap: 18px; }
    label.field { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: var(--muted); }
    input[type="text"], input[type="number"] {
      background: var(--bar-bg); border: 1px solid var(--border); color: var(--text);
      border-radius: 8px; padding: 8px 10px; font-size: 14px; font-family: inherit;
    }
    input:focus { outline: none; border-color: var(--accent); }
    input:disabled { opacity: 0.4; }
    fieldset { border: 1px solid var(--border); border-radius: 8px; padding: 12px 14px; margin: 0; }
    legend { font-size: 12px; color: var(--muted); padding: 0 6px; }
    .targets { display: flex; flex-wrap: wrap; gap: 14px; }
    .target { display: flex; align-items: center; gap: 6px; font-size: 14px; color: var(--text); }

    details.group { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
    details.group + details.group { margin-top: 10px; }
    summary { list-style: none; cursor: pointer; padding: 10px 14px; display: flex; align-items: center; gap: 10px; background: var(--bar-bg); }
    summary::-webkit-details-marker { display: none; }
    summary .gname { font-size: 14px; font-weight: 600; color: var(--text); }
    summary .gcount { font-size: 12px; color: var(--muted); margin-left: auto; }
    .gdesc { padding: 8px 14px 0; font-size: 12px; color: var(--muted); }
    .viz { padding: 10px 14px; border-bottom: 1px solid var(--border); }
    .viz:empty { display: none; }

    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th { text-align: left; color: var(--muted); font-weight: 500; padding: 6px 14px; border-bottom: 1px solid var(--border); }
    td { padding: 6px 14px; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: top; }
    td.val input { width: 110px; }
    tr.off td { color: var(--muted); }
    tr.over td { background: color-mix(in srgb, var(--accent) 9%, transparent); }
    .plabel { color: var(--text); }
    .pdesc { color: var(--muted); font-size: 11px; margin-top: 2px; max-width: 320px; }
    .pcode { color: var(--muted); font-variant-numeric: tabular-nums; font-size: 11px; }
    .def { color: var(--muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
    .unit { color: var(--muted); }
    .otag {
      margin-left: 8px; font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em;
      color: var(--accent); border: 1px solid color-mix(in srgb, var(--accent) 55%, transparent); border-radius: 999px; padding: 1px 6px;
    }
    .rotag {
      margin-left: 8px; font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em;
      color: var(--muted); border: 1px solid var(--border); border-radius: 999px; padding: 1px 6px;
    }
    .src { font-size: 10px; color: var(--muted); border: 1px solid var(--border); border-radius: 4px; padding: 0 3px; cursor: help; }
    button.clear {
      margin-left: 6px; background: transparent; border: 1px solid var(--border); color: var(--muted);
      border-radius: 6px; width: 24px; height: 24px; font-size: 12px; cursor: pointer; vertical-align: middle;
    }
    button.clear:hover { color: var(--text); border-color: var(--muted); }
    .warn { display: block; margin-top: 4px; font-size: 11px; color: var(--warn); }

    .conflicts { border-radius: 8px; padding: 10px 12px; font-size: 13px; color: var(--err);
      border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .conflicts ul { margin: 6px 0 0; padding-left: 18px; }

    .actions { display: flex; align-items: center; gap: 12px; }
    button { border-radius: 8px; padding: 9px 18px; font-size: 14px; font-weight: 600; cursor: pointer; border: none; }
    button.save { background: var(--accent); color: #04121a; }
    button.save:hover:not(:disabled) { filter: brightness(1.08); }
    button.cancel { background: transparent; border: 1px solid var(--border); color: var(--text); }
    button:disabled { opacity: 0.45; cursor: not-allowed; }
    .err { color: var(--err); font-size: 13px; }
    .hint { color: var(--muted); font-size: 12px; }
  `;willUpdate(Q){if(Q.has("profile")){let Y=this.profile;this.name=Y?.id??"",this.selectedUids=[...Y?.uids??[]];let K={};for(let X of Y?.points??[])K[X.aps_code]=String(X.value);this.values=K,this.localError=""}}effectiveWritable(){if(!this.selectedUids.length)return new Set;let Q=this.selectedUids.map((K)=>new Set(this.inverters.find((X)=>X.uid===K)?.writable_codes??[])),Y=Q[0];for(let K of Q.slice(1))Y=new Set([...Y].filter((X)=>K.has(X)));return Y}targetDefault(Q){let Y=this.defaults[Q];if(Y)return{value:Y.value,source:"base"};if(!this.selectedUids.length)return;let K;for(let X of this.selectedUids){let G=this.inverters.find((z)=>z.uid===X)?.current?.[Q];if(G===void 0)return;if(K===void 0)K=G;else if(Math.abs(G-K)>0.000001)return}return K===void 0?void 0:{value:K,source:"inverter"}}effectiveValue(Q){let Y=(this.values[Q]??"").trim();if(Y!==""&&!Number.isNaN(Number(Y)))return Number(Y);return this.targetDefault(Q)?.value}isOverride(Q){let Y=(this.values[Q]??"").trim();if(Y===""||Number.isNaN(Number(Y)))return!1;let K=this.targetDefault(Q);return!K||Number(Y)!==K.value}prefill(Q){if((this.values[Q]??"").trim()!=="")return;let Y=this.targetDefault(Q);if(Y)this.setValue(Q,Z(Y.value))}outOfRange(Q){let Y=(this.values[Q]??"").trim();if(Y===""||Number.isNaN(Number(Y)))return!1;let K=this.defaults[Q];if(!K)return!1;let X=Number(Y);return K.min!==void 0&&X<K.min||K.max!==void 0&&X>K.max}label(Q){return this.names[Q.uid]||Q.model||Q.uid}toggleTarget(Q,Y){this.selectedUids=Y?[...this.selectedUids,Q]:this.selectedUids.filter((K)=>K!==Q)}setValue(Q,Y){this.values={...this.values,[Q]:Y}}groups(){let Q={};for(let K of this.params)(Q[K.group]??=[]).push(K);return[...L3,...Object.keys(Q).filter((K)=>!L3.includes(K))].filter((K)=>Q[K]?.length).map((K)=>[K,Q[K]])}save=()=>{let Q=this.effectiveWritable(),Y=this.params.filter((X)=>Q.has(X.aps_code)&&this.isOverride(X.aps_code)).map((X)=>({aps_code:X.aps_code,value:Number(this.values[X.aps_code])}));if(!this.name.trim())return void(this.localError="Profile name is required.");if(!this.selectedUids.length)return void(this.localError="Select at least one target inverter.");if(!Y.length)return void(this.localError="Change at least one parameter from its default.");if(F3(this.rules,(X)=>this.effectiveValue(X)).length)return void(this.localError="Resolve the conflicts before saving.");this.localError="";let K={id:this.name.trim(),uids:this.selectedUids,points:Y};this.dispatchEvent(new CustomEvent("save",{detail:K,bubbles:!0,composed:!0}))};cancel=()=>this.dispatchEvent(new CustomEvent("cancel",{bubbles:!0,composed:!0}));markers(Q,Y){let K=[];for(let X of this.params){if(X.group!==Q||X.unit!==Y)continue;if(X.polarity!=="under"&&X.polarity!=="over")continue;let G=this.effectiveValue(X.aps_code);if(G!==void 0)K.push({value:G,label:X.aps_code,kind:X.polarity})}return K}vizFor(Q){if(Q==="DERFreqDroop")return q`<freq-watt-chart
        .deadband=${this.effectiveValue("CA")}
        .slope=${this.effectiveValue("DD")}
        .trip=${this.effectiveValue("AF")}
        .nominal=${50}
      ></freq-watt-chart>`;if(Q==="CrvSet"){let Y=this.markers(Q,"Hz");return Y.length?q`<trip-line unit="Hz" .nominal=${50} .markers=${Y}></trip-line>`:J}if(Q==="MustTrip"){let Y=this.markers(Q,"V"),K=this.markers(Q,"Hz");return q`
        ${Y.length?q`<trip-line unit="V" .nominal=${230} .markers=${Y}></trip-line>`:J}
        ${K.length?q`<trip-line unit="Hz" .nominal=${50} .markers=${K}></trip-line>`:J}
      `}return J}renderRow(Q,Y){let K=Y.has(Q.aps_code),X=this.targetDefault(Q.aps_code),G=this.defaults[Q.aps_code],z=(this.values[Q.aps_code]??"").trim(),B=this.isOverride(Q.aps_code),H=K&&this.outOfRange(Q.aps_code),W=K?this.values[Q.aps_code]??"":X?Z(X.value):"";return q`<tr class="${K?"":"off"} ${B?"over":""}">
      <td>
        <div class="plabel">
          ${YQ(Q.aps_code,Q.long_name)}
          ${B?q`<span class="otag">overridden</span>`:J}
          ${!K&&X?q`<span class="rotag">read-only</span>`:J}
        </div>
        <div class="pdesc">${KQ(Q.aps_code)}</div>
      </td>
      <td class="pcode">${Q.aps_code}</td>
      <td class="def">
        ${X?q`${Z(X.value)} ${Q.unit}${X.source==="inverter"?q` <span class="src" title="from the inverter's current value">inv</span>`:J}`:"—"}
      </td>
      <td class="val">
        <input
          type="number" step="any" ?disabled=${!K}
          .value=${W}
          placeholder=${X?Z(X.value):K?"—":"n/a"}
          @focus=${()=>this.prefill(Q.aps_code)}
          @input=${(U)=>this.setValue(Q.aps_code,U.target.value)}
        />
        <span class="unit">${Q.unit}</span>
        ${K&&z!==""?q`<button class="clear" title="Clear override" @click=${()=>this.setValue(Q.aps_code,"")}>↺</button>`:J}
        ${H?q`<span class="warn">⚠ outside base range${G?.min!==void 0?` (${Z(G.min)}–${Z(G.max)} ${Q.unit})`:""}</span>`:J}
      </td>
    </tr>`}render(){let Q=this.effectiveWritable(),Y=this.selectedUids.length>0,K=Y?F3(this.rules,(X)=>this.effectiveValue(X)):[];return q`
      <div class="grid">
        <label class="field">
          Profile name
          <input type="text" .value=${this.name} ?disabled=${this.editing} placeholder="e.g. victron-shift"
            @input=${(X)=>this.name=X.target.value} />
        </label>

        <fieldset>
          <legend>Target inverters</legend>
          <div class="targets">
            ${this.inverters.length===0?q`<span class="hint">No inverters seen yet.</span>`:this.inverters.map((X)=>q`<label class="target">
                    <input type="checkbox" .checked=${this.selectedUids.includes(X.uid)}
                      @change=${(G)=>this.toggleTarget(X.uid,G.target.checked)} />
                    ${this.label(X)} <span class="pcode">${X.model}</span>
                  </label>`)}
          </div>
        </fieldset>

        ${!Y?q`<span class="hint">Select a target to choose editable parameters.</span>`:q`
              ${K.length?q`<div class="conflicts">⚠ Conflicting settings — resolve to save:
                    <ul>${K.map((X)=>q`<li>${X}</li>`)}</ul>
                  </div>`:J}

              ${this.groups().map(([X,G])=>{let z=e9[X];return q`<details class="group" ?open=${!QQ.has(X)}>
                  <summary>
                    <span class="gname">${z?.label??X}</span>
                    <span class="gcount">${G.length} setting${G.length===1?"":"s"}</span>
                  </summary>
                  ${z?.tip?q`<div class="gdesc">${z.tip}</div>`:J}
                  <div class="viz">${this.vizFor(X)}</div>
                  <table>
                    <thead><tr><th>Setting</th><th>Code</th><th>Default</th><th>Override</th></tr></thead>
                    <tbody>${G.map((B)=>this.renderRow(B,Q))}</tbody>
                  </table>
                </details>`})}

              ${this.selectedUids.length>1?q`<div class="hint">Greyed rows are not writable on every selected target.</div>`:J}
            `}

        ${this.localError?q`<div class="err">⚠ ${this.localError}</div>`:J}

        <div class="actions">
          <button class="save" @click=${this.save} ?disabled=${this.busy||K.length>0}>
            ${this.busy?"Applying…":"Save & apply"}
          </button>
          <button class="cancel" @click=${this.cancel} ?disabled=${this.busy}>Cancel</button>
          <span class="hint">${K.length?"resolve conflicts to save":"applies to the selected inverters"}</span>
        </div>
      </div>
    `}}customElements.define("local-site-profile-form",qQ);class zQ extends ${static properties={data:{state:!0},names:{state:!0},error:{state:!0},notice:{state:!0},baseBusy:{state:!0},overlayBusy:{state:!0},editing:{state:!0},editingExisting:{state:!0}};constructor(){super();this.data=null,this.names={},this.error="",this.notice="",this.baseBusy=!1,this.overlayBusy=!1,this.editing=null,this.editingExisting=!1}static styles=L`
    :host { display: block; }
    .cols {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 320px;
      gap: 20px;
      align-items: start;
      max-width: 1200px;
    }
    @media (max-width: 900px) { .cols { grid-template-columns: 1fr; } }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      margin-bottom: 20px;
      min-width: 0;
    }
    h2 { font-size: 15px; margin: 0 0 16px; color: var(--text); }
    .row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
    .hdr-actions { display: flex; gap: 10px; }
    button.primary { background: var(--accent); border: none; color: #04121a; border-radius: 8px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; }
    button.primary:hover { filter: brightness(1.08); }
    button.ghost { background: transparent; border: 1px solid var(--border); color: var(--text); border-radius: 8px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; }
    button.ghost:hover { border-color: var(--muted); }
    .cards { display: grid; gap: 12px; }
    .card { border: 1px solid var(--border); border-radius: 8px; padding: 14px 16px; }
    .card .title { font-size: 14px; font-weight: 600; color: var(--text); }
    .card .meta { font-size: 12px; color: var(--muted); margin-top: 4px; }
    .chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
    .chip { font-size: 12px; background: var(--bar-bg); border: 1px solid var(--border); border-radius: 12px; padding: 2px 9px; color: var(--text); }
    .cardactions { display: flex; gap: 10px; margin-top: 12px; }
    .cardactions button { font-size: 12px; border-radius: 6px; padding: 5px 12px; cursor: pointer; border: 1px solid var(--border); background: transparent; color: var(--text); }
    .cardactions button.del { color: var(--err); border-color: color-mix(in srgb, var(--err) 50%, var(--border)); }
    .empty { color: var(--muted); font-size: 13px; }
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){try{let[Q,Y]=await Promise.all([D.profiles(),D.getSettings()]);this.data=Q,this.error=Q.error??"",this.names=Y.settings?.inverter_names??{}}catch(Q){this.error=Q.message}}invName(Q){if(this.names[Q])return this.names[Q];return this.data?.inverters.find((K)=>K.uid===Q)?.model||Q}onSelectBase=async(Q)=>{let Y=Q.detail;if(!window.confirm(`Apply base grid profile "${Y}" to every inverter? This writes grid-protection settings across the whole fleet.`))return;this.baseBusy=!0,this.notice="",this.error="";try{await D.selectBase(Y),await this.load(),this.notice=`Base profile "${Y}" selected — reconciling the fleet now. See Events for per-inverter progress and results.`}catch(K){this.error=K.message}finally{this.baseBusy=!1}};newProfile(){this.editing={id:"",uids:[],points:[]},this.editingExisting=!1,this.notice="",this.error=""}editProfile(Q){this.editing=Q,this.editingExisting=!0,this.notice="",this.error=""}onCancelEdit=()=>{this.editing=null};exportProfile(Q){let Y={id:Q.id,uids:Q.uids,points:Q.points.map((z)=>({aps_code:z.aps_code,value:z.value}))},K=new Blob([JSON.stringify(Y,null,2)],{type:"application/json"}),X=URL.createObjectURL(K),G=document.createElement("a");G.href=X,G.download=`${Q.id||"profile"}.json`,G.click(),URL.revokeObjectURL(X)}triggerImport=()=>{this.shadowRoot?.querySelector("#importfile")?.click()};onImportFile=async(Q)=>{let Y=Q.target,K=Y.files?.[0];if(Y.value="",!K)return;let X;try{X=JSON.parse(await K.text())}catch(H){this.error="Import failed: "+H.message;return}let G=s9(X);if(!G.ok){let H=G.errors.slice(0,3).join("; "),W=G.errors.length>3?` (+${G.errors.length-3} more)`:"";this.error="Import failed: "+H+W;return}this.editing=G.profile,this.editingExisting=!1,this.error="";let z=`Imported "${G.profile.id}" — review the targets and values, then Save.`,B=G.warnings.length>0?` — Note: ${G.warnings.join("; ")}`:"";this.notice=z+B};onSaveOverlay=async(Q)=>{let Y=Q.detail;if(!window.confirm(`Apply Local Site profile "${Y.id}" to ${Y.uids.length} inverter(s)? This writes grid-protection parameters to each.`))return;this.overlayBusy=!0,this.notice="",this.error="";try{let K=await D.saveOverlay(Y);this.editing=null,await this.load();let X=K.uids.length;this.notice=`Overlay "${K.id}" queued for ${X} inverter${X===1?"":"s"} — see Events for application results.`}catch(K){this.error=K.message}finally{this.overlayBusy=!1}};deleteProfile=async(Q)=>{if(!window.confirm(`Delete Local Site profile "${Q.id}" and clear it from ${Q.uids.length} inverter(s)?`))return;this.overlayBusy=!0,this.notice="",this.error="";try{let Y=await D.deleteOverlay(Q.id,Q.uids);if(this.editing?.id===Q.id)this.editing=null;await this.load();let K=Y.uids.length,X=`Profile "${Q.id}" cleared from ${K} inverter${K===1?"":"s"} — reconciling back to the base profile now. See Events for results.`;if(Y.failed&&Y.failed.length>0){let G=Y.failed.map((z)=>`${this.invName(z.uid)}: ${z.error||"rejected"}`).join("; ");X+=` Not queued on ${Y.failed.length} inverter(s): ${G}`}this.notice=X}catch(Y){this.error=Y.message}finally{this.overlayBusy=!1}};renderBase(){let Q=this.data?.base;return q`
      <div class="panel">
        <h2>Base grid profile</h2>
        <grid-profile-form
          .profiles=${Q?.profiles??[]}
          .activeBase=${Q?.active_base??""}
          .reconcilerReady=${Q?.reconciler_ready??!1}
          .busy=${this.baseBusy}
          @apply=${this.onSelectBase}
        ></grid-profile-form>
      </div>
    `}renderLocalSite(){let Q=this.data;return q`
      <div class="panel">
        <div class="row">
          <h2 style="margin:0">Local Site profiles</h2>
          ${this.editing===null?q`<div class="hdr-actions">
                <button class="ghost" @click=${this.triggerImport}>Import</button>
                <button class="primary" @click=${()=>this.newProfile()}>+ New profile</button>
              </div>`:J}
        </div>
        <input id="importfile" type="file" accept=".json,application/json" hidden @change=${this.onImportFile} />

        ${this.editing!==null?q`<local-site-profile-form
              .params=${Q?.params??[]}
              .inverters=${Q?.inverters??[]}
              .defaults=${Q?.base_defaults??{}}
              .rules=${Q?.conflict_rules??[]}
              .names=${this.names}
              .profile=${this.editing}
              .editing=${this.editingExisting}
              .busy=${this.overlayBusy}
              @save=${this.onSaveOverlay}
              @cancel=${this.onCancelEdit}
            ></local-site-profile-form>`:this.renderCards()}
      </div>
    `}renderCards(){let Q=this.data?.overlays??[];if(Q.length===0)return q`<div class="empty">No Local Site profiles yet. Create one to override grid-protection parameters on specific inverters.</div>`;return q`<div class="cards">
      ${Q.map((Y)=>q`<div class="card">
          <div class="title">${Y.id}</div>
          <div class="meta">Targets: ${Y.uids.map((K)=>this.invName(K)).join(", ")||"none"}</div>
          <div class="chips">
            ${Y.points.map((K)=>q`<span class="chip">${K.aps_code} = ${Z(K.value)}${K.unit?` ${K.unit}`:""}</span>`)}
          </div>
          <div class="cardactions">
            <button @click=${()=>this.editProfile(Y)}>Edit</button>
            <button @click=${()=>this.exportProfile(Y)}>Export</button>
            <button class="del" @click=${()=>this.deleteProfile(Y)}>Delete</button>
          </div>
        </div>`)}
    </div>`}render(){return q`
      ${this.notice?q`<div class="banner ok">${this.notice}</div>`:J}
      ${this.error?q`<div class="banner err">⚠ ${this.error}</div>`:J}
      ${this.data===null?q`<div class="panel"><div class="loading">Loading…</div></div>`:q`<div class="cols">
            <div>${this.renderLocalSite()}</div>
            <div>${this.renderBase()}</div>
          </div>`}
    `}}customElements.define("profiles-view",zQ);var BQ=16;class jQ extends ${static properties={settings:{attribute:!1},hostname:{attribute:!1},confirming:{state:!0},pendingDetail:{state:!0},pwdError:{state:!0},pwdBusy:{state:!0},typedMac:{state:!0},typedPan:{state:!0},typedChannel:{state:!0}};constructor(){super();this.settings={ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"},this.hostname="",this.confirming=!1,this.pendingDetail=null,this.pwdError="",this.pwdBusy=!1,this.typedMac="",this.typedPan="",this.typedChannel=""}willUpdate(Q){if(Q.has("settings"))this.typedMac=this.settings.mac??"",this.typedPan=this.settings.pan_override??"",this.typedChannel=WQ(this.settings.channel)}static styles=L`
    :host { display: block; }
    .grid { display: grid; gap: 18px; max-width: 460px; }
    label { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: var(--muted); }
    input, select {
      background: var(--bar-bg);
      border: 1px solid var(--border);
      color: var(--text);
      border-radius: 8px;
      padding: 9px 11px;
      font-size: 14px;
      font-family: inherit;
    }
    input:focus, select:focus { outline: none; border-color: var(--accent); }
    .hint { font-size: 12px; color: var(--muted); margin-top: -2px; }
    .err-inline {
      font-size: 12px;
      color: var(--err);
      margin-top: -2px;
    }
    .banner.err {
      color: var(--err);
      border: 1px solid var(--err);
      background: color-mix(in srgb, var(--err) 12%, transparent);
      border-radius: 8px;
      padding: 9px 11px;
      font-size: 13px;
    }
    button.save:disabled {
      opacity: 0.55;
      cursor: not-allowed;
    }
    .actions { display: flex; gap: 12px; margin-top: 4px; }
    button.save {
      background: var(--accent);
      border: none;
      color: #04121a;
      border-radius: 8px;
      padding: 9px 18px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
    }
    button.save:hover { filter: brightness(1.08); }

    .backdrop {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.55);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }
    .dialog {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 20px 22px;
      max-width: 440px;
      width: 92%;
      color: var(--text);
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
    }
    .dialog h3 { margin: 0 0 10px; font-size: 15px; }
    .dialog p { margin: 0 0 14px; font-size: 13px; color: var(--muted); line-height: 1.45; }
    .dialog label { display: block; font-size: 12px; color: var(--muted); margin: 4px 0 6px; }
    .dialog input {
      width: 100%;
      box-sizing: border-box;
      padding: 9px 11px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font: inherit;
    }
    .dialog .err {
      color: var(--err);
      border: 1px solid var(--err);
      background: color-mix(in srgb, var(--err) 12%, transparent);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      margin-top: 10px;
    }
    .dialog p.warn {
      color: var(--text);
      border: 1px solid var(--accent);
      background: color-mix(in srgb, var(--accent) 10%, transparent);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      margin: 0 0 12px;
    }
    .dialog .row {
      display: flex;
      gap: 10px;
      justify-content: flex-end;
      margin-top: 16px;
    }
    .dialog button {
      padding: 8px 14px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    .dialog button.primary { background: var(--accent); color: #04121a; border: none; }
    .dialog button.secondary { background: transparent; color: var(--muted); border: 1px solid var(--border); }
    .dialog button:disabled { opacity: 0.6; cursor: default; }
  `;currentDetail(){let Q=this.shadowRoot,Y=(K)=>(Q?.querySelector(`#${K}`)?.value??"").trim();return{ecu_id:Y("ecu_id"),mac:Y("mac"),pan_override:Y("pan_override"),zigbee_type:Y("zigbee_type"),channel:b8(Y("channel"))}}computeEffectivePAN(Q){let Y=JQ(Q.pan_override);if(Y)return Y;return HQ(Q.mac||"")}effectivePAN(){return this.computeEffectivePAN(this.settings)}effectiveChannel(){let Q=this.settings.channel;return Q&&Q>0?Q:BQ}sensitiveChange(Q){return Q.mac!==(this.settings.mac??"")||Q.pan_override!==(this.settings.pan_override??"")}macInputInvalid(Q){if(Q.mac===(this.settings.mac??""))return!1;return Q.mac!==""&&!I3(Q.mac)}save=()=>{if(UQ(this.typedChannel))return;let Q=this.currentDetail(),Y=this.sensitiveChange(Q);if(Y&&this.macInputInvalid(Q))return;let K=this.effectivePAN(),X=this.computeEffectivePAN(Q);if(Y&&!X)return;if(K&&X&&X!==K){this.pendingDetail=Q,this.pwdError="",this.confirming=!0,queueMicrotask(()=>{this.shadowRoot?.querySelector("#confirm_pwd")?.focus()});return}this.dispatchSave(Q)};dispatchSave(Q){this.dispatchEvent(new CustomEvent("save",{detail:Q,bubbles:!0,composed:!0}))}confirmCancel=()=>{this.confirming=!1,this.pendingDetail=null,this.pwdError="",this.pwdBusy=!1};confirmSubmit=async()=>{if(this.pwdBusy)return;let Y=this.shadowRoot?.querySelector("#confirm_pwd")?.value??"";if(!Y){this.pwdError="Password required.";return}this.pwdBusy=!0,this.pwdError="";try{if(!await D.verifyPassword(Y)){this.pwdError="Wrong password.";return}let X=this.pendingDetail;if(this.confirming=!1,this.pendingDetail=null,X)this.dispatchSave(X)}catch(K){this.pwdError=K.message||"Verification failed."}finally{this.pwdBusy=!1}};onPwdKey=(Q)=>{if(Q.key==="Enter")Q.preventDefault(),this.confirmSubmit()};render(){let Q=this.settings,Y="e.g. the serial on the device label",K=Q.ecu_id||this.hostname||"",X=this.effectivePAN(),G=this.effectiveChannel(),z=Q.mac?`effective PAN source: ${Q.mac}`:"",B=Q.pan_override?X?`effective: ${X}`:"":X?`effective: ${X} (from MAC)`:"",H=Q.zigbee_type?"":"effective: apsystems (default)",W=`effective: ${G}`,U=this.typedMac!==(Q.mac??""),j=this.typedPan!==(Q.pan_override??""),A=U||j,F=U&&this.typedMac!==""&&!I3(this.typedMac),I=!!JQ(this.typedPan)||!!HQ(this.typedMac),P=A&&!I,N=UQ(this.typedChannel),c=N||A&&(F||P),g="";if(F)g="MAC must be 6 colon-separated hex octets (e.g. aa:bb:cc:dd:ee:ff).";else if(A&&P)g="Cannot resolve effective PAN; refusing to save MAC / PAN-override changes.";return q`
      <div class="grid">
        <label>
          ECU ID
          <input
            id="ecu_id"
            type="text"
            placeholder=${"e.g. the serial on the device label"}
            .value=${K}
          />
          ${!Q.ecu_id?q`<div class="hint">Recommended: use the serial on the device label.</div>`:J}
        </label>
        <label>
          MAC
          <input
            id="mac"
            type="text"
            placeholder="aa:bb:cc:dd:ee:ff"
            pattern="^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$"
            .value=${Q.mac??""}
            @input=${this.onMacInput}
          />
          ${z?q`<div class="hint">${z}</div>`:J}
          ${F?q`<div class="err-inline">Use colon-separated hex (e.g. aa:bb:cc:dd:ee:ff).</div>`:J}
        </label>
        <label>
          PAN override
          <input
            id="pan_override"
            type="text"
            placeholder="auto from MAC"
            .value=${Q.pan_override??""}
            @input=${this.onPanInput}
          />
          ${B?q`<div class="hint">${B}</div>`:J}
        </label>
        <label>
          ZigBee channel
          <input
            id="channel"
            type="number"
            min="11"
            max="26"
            step="1"
            placeholder=${`auto (${BQ})`}
            .value=${WQ(Q.channel)}
            @input=${this.onChannelInput}
          />
          ${W?q`<div class="hint">${W}</div>`:J}
          ${N?q`<div class="err-inline">Channel must be empty (auto) or an integer 11–26.</div>`:J}
        </label>
        <label>
          ZigBee type
          <select id="zigbee_type" .value=${Q.zigbee_type||"apsystems"}>
            <option value="apsystems">apsystems</option>
            <option value="general">general</option>
          </select>
          ${H?q`<div class="hint">${H}</div>`:J}
        </label>
        ${g?q`<div class="banner err">${g}</div>`:J}
        <div class="actions">
          <button class="save" ?disabled=${c} @click=${this.save}>
            Save
          </button>
        </div>
      </div>
      ${this.confirming?this.renderDialog():J}
    `}onMacInput=(Q)=>{this.typedMac=Q.target.value.trim()};onPanInput=(Q)=>{this.typedPan=Q.target.value.trim()};onChannelInput=(Q)=>{this.typedChannel=Q.target.value.trim()};renderDialog(){let Q=this.effectivePAN(),Y=this.pendingDetail?this.computeEffectivePAN(this.pendingDetail):"",K=!!this.pendingDetail&&(this.pendingDetail.mac??"")!==(this.settings.mac??"");return q`
      <div class="backdrop" @click=${this.onBackdropClick}>
        <div class="dialog" role="dialog" aria-modal="true" @click=${this.stop}>
          <h3>Confirm PAN change</h3>
          <p>
            Effective PAN ${Q||"—"} → ${Y||"—"}. Inverters bonded to
            ${Q||"the current PAN"} may stop responding.
          </p>
          ${K?q`<p class="warn">
                Applying a new MAC drops the network for a few seconds, up to
                ~15 s if the kernel is slow. Your browser may reconnect
                automatically; if not, refresh.
              </p>`:J}
          <label for="confirm_pwd">Password</label>
          <input
            id="confirm_pwd"
            type="password"
            autocomplete="current-password"
            @keydown=${this.onPwdKey}
            ?disabled=${this.pwdBusy}
          />
          ${this.pwdError?q`<div class="err">${this.pwdError}</div>`:J}
          <div class="row">
            <button class="secondary" @click=${this.confirmCancel} ?disabled=${this.pwdBusy}>
              Cancel
            </button>
            <button class="primary" @click=${this.confirmSubmit} ?disabled=${this.pwdBusy}>
              Confirm
            </button>
          </div>
        </div>
      </div>
    `}onBackdropClick=()=>this.confirmCancel();stop=(Q)=>Q.stopPropagation()}function JQ(Q){let Y=(Q||"").trim().replace(/^0x/i,"");if(!Y)return"";if(!/^[0-9a-fA-F]{1,4}$/.test(Y))return"";return Y.toUpperCase().padStart(4,"0")}function HQ(Q){let Y=(Q||"").trim();if(!Y||!I3(Y))return"";return Y.replace(/:/g,"").slice(-4).toUpperCase()}function I3(Q){return/^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$/.test(Q)}function WQ(Q){return Q&&Q>0?String(Q):""}function b8(Q){let Y=(Q||"").trim();if(!Y)return 0;let K=Number(Y);return Number.isInteger(K)?K:0}function UQ(Q){let Y=(Q||"").trim();if(!Y)return!1;let K=Number(Y);if(!Number.isInteger(K))return!0;return K<11||K>26}customElements.define("settings-form",jQ);class $Q extends ${static properties={pwError:{state:!0},pwNotice:{state:!0},pwBusy:{state:!0},recError:{state:!0},recBusy:{state:!0},newCode:{state:!0}};constructor(){super();this.pwError="",this.pwNotice="",this.pwBusy=!1,this.recError="",this.recBusy=!1,this.newCode=""}static styles=L`
    :host { display: block; }
    h3 { font-size: 13px; margin: 0 0 12px; color: var(--text); }
    .section + .section { margin-top: 24px; padding-top: 20px; border-top: 1px solid var(--border); }
    label { display: block; font-size: 12px; color: var(--muted); margin: 12px 0 6px; }
    label:first-of-type { margin-top: 0; }
    input {
      width: 100%;
      box-sizing: border-box;
      padding: 9px 12px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font-size: 14px;
    }
    button {
      margin-top: 14px;
      padding: 9px 16px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    button.primary { background: var(--accent); color: #04222b; border: none; }
    button.secondary { background: transparent; color: var(--muted); border: 1px solid var(--border); }
    button.secondary:hover { color: var(--text); border-color: var(--muted); }
    button:disabled { opacity: 0.6; cursor: default; }
    .muted { color: var(--muted); font-size: 13px; margin: 0 0 6px; }
    .banner { border-radius: 8px; padding: 8px 12px; font-size: 13px; margin-top: 12px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .code {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 16px;
      letter-spacing: 0.06em;
      text-align: center;
      background: var(--bar-bg);
      border: 1px solid var(--accent);
      border-radius: 8px;
      padding: 12px;
      color: var(--text);
      user-select: all;
      margin-top: 12px;
    }
  `;val(Q){return this.renderRoot.querySelector(`#${Q}`)?.value??""}clear(Q){let Y=this.renderRoot.querySelector(`#${Q}`);if(Y)Y.value=""}changePassword=async(Q)=>{if(Q.preventDefault(),this.pwBusy)return;if(this.pwError="",this.pwNotice="",this.val("new")!==this.val("new2")){this.pwError="New passwords do not match.";return}this.pwBusy=!0;try{await D.changePassword(this.val("cur"),this.val("new")),this.pwNotice="Password changed.",this.clear("cur"),this.clear("new"),this.clear("new2")}catch(Y){this.pwError=Y.message||"failed"}finally{this.pwBusy=!1}};regenerate=async()=>{if(this.recBusy)return;this.recError="",this.newCode="",this.recBusy=!0;try{let Q=await D.regenerateRecovery();this.newCode=Q.recovery_code}catch(Q){this.recError=Q.message||"failed"}finally{this.recBusy=!1}};render(){return q`
      <div class="section">
        <h3>Change password</h3>
        <form @submit=${this.changePassword}>
          <label for="cur">Current password</label>
          <input id="cur" type="password" autocomplete="current-password" ?disabled=${this.pwBusy} />
          <label for="new">New password</label>
          <input id="new" type="password" autocomplete="new-password" ?disabled=${this.pwBusy} />
          <label for="new2">Confirm new password</label>
          <input id="new2" type="password" autocomplete="new-password" ?disabled=${this.pwBusy} />
          <button class="primary" type="submit" ?disabled=${this.pwBusy}>
            ${this.pwBusy?"…":"Change password"}
          </button>
          ${this.pwNotice?q`<div class="banner ok">${this.pwNotice}</div>`:J}
          ${this.pwError?q`<div class="banner err">⚠ ${this.pwError}</div>`:J}
        </form>
      </div>

      <div class="section">
        <h3>Recovery code</h3>
        <p class="muted">
          The recovery code resets your password without console access. Generating a
          new one invalidates the previous code. It's shown only once.
        </p>
        <button class="secondary" type="button" @click=${this.regenerate} ?disabled=${this.recBusy}>
          ${this.recBusy?"…":"Generate new recovery code"}
        </button>
        ${this.newCode?q`<div class="code">${this.newCode}</div>
              <p class="muted" style="margin-top:8px">Write this down now — it won't be shown again.</p>`:J}
        ${this.recError?q`<div class="banner err">⚠ ${this.recError}</div>`:J}
      </div>
    `}}customElements.define("account-security-form",$Q);class AQ extends ${static properties={settings:{state:!0},hostname:{state:!0},error:{state:!0},notice:{state:!0},loading:{state:!0},saving:{state:!0}};constructor(){super();this.settings=null,this.hostname="",this.error="",this.notice="",this.loading=!1,this.saving=!1}static styles=L`
    :host { display: block; }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      max-width: 560px;
    }
    .panel + .panel { margin-top: 22px; }
    h2 { font-size: 15px; margin: 0 0 16px; color: var(--text); }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
  `;connectedCallback(){super.connectedCallback(),this.load(),this.loadHostname()}async load(){this.loading=!0;try{let Q=await D.getSettings();this.settings=Q.settings??null,this.error=Q.error??""}catch(Q){this.error=Q.message}finally{this.loading=!1}}async loadHostname(){try{let Q=await D.system();this.hostname=Q?.ecu?.hostname??""}catch{this.hostname=""}}onSave=async(Q)=>{this.saving=!0,this.notice="",this.error="";try{this.settings=await D.saveSettings(Q.detail),this.notice="Settings saved."}catch(Y){this.error=Y.message}finally{this.saving=!1,await this.load()}};render(){return q`
      <div class="panel">
        <h2>ECU settings</h2>
        ${this.notice?q`<div class="banner ok">${this.notice}</div>`:J}
        ${this.error?q`<div class="banner err">⚠ ${this.error}</div>`:J}
        ${this.loading&&!this.settings?q`<div class="loading">Loading…</div>`:q`<settings-form
              .settings=${this.settings??{ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}
              .hostname=${this.hostname}
              @save=${this.onSave}
            ></settings-form>`}
      </div>
      <div class="panel">
        <h2>Account &amp; security</h2>
        <account-security-form></account-security-form>
      </div>
    `}}customElements.define("settings-view",AQ);class DQ extends ${static properties={state:{state:!0},loading:{state:!0},error:{state:!0},notice:{state:!0},adding:{state:!0},addError:{state:!0},pendingFp:{state:!0},pwError:{state:!0},deleting:{state:!0}};constructor(){super();this.state=null,this.loading=!1,this.error="",this.notice="",this.adding=!1,this.addError="",this.pendingFp="",this.pwError="",this.deleting=!1}static styles=L`
    :host { display: block; }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      max-width: 680px;
    }
    .panel + .panel { margin-top: 22px; }
    h2 { font-size: 15px; margin: 0 0 4px; color: var(--text); }
    .sub { color: var(--muted); font-size: 12px; margin: 0 0 16px; }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
    .nudge {
      border: 1px dashed var(--border);
      border-radius: 8px;
      padding: 16px;
      color: var(--muted);
      font-size: 13px;
      text-align: center;
    }
    ul.keys { list-style: none; margin: 0; padding: 0; }
    li.key {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 12px;
      padding: 12px 0;
      border-top: 1px solid var(--border);
    }
    li.key:first-child { border-top: none; }
    .keymeta { min-width: 0; }
    .comment { color: var(--text); font-size: 14px; font-weight: 600; }
    .comment.none { color: var(--muted); font-weight: 400; font-style: italic; }
    .fp {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 12px;
      color: var(--muted);
      word-break: break-all;
      margin-top: 3px;
    }
    .added { color: var(--muted); font-size: 12px; margin-top: 3px; }
    label { display: block; font-size: 12px; color: var(--muted); margin: 14px 0 6px; }
    textarea, input {
      width: 100%;
      box-sizing: border-box;
      padding: 9px 12px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font-size: 14px;
    }
    textarea {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 12px;
      resize: vertical;
      min-height: 64px;
    }
    button {
      margin-top: 14px;
      padding: 9px 16px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    button.primary { background: var(--accent); color: #04222b; border: none; }
    button.danger {
      margin-top: 0;
      background: transparent;
      color: var(--err);
      border: 1px solid var(--err);
      padding: 6px 12px;
      flex: none;
    }
    button.danger:hover { background: color-mix(in srgb, var(--err) 12%, transparent); }
    button:disabled { opacity: 0.6; cursor: default; }
    .addrow { border-top: 1px solid var(--border); margin-top: 18px; padding-top: 6px; }
    /* step-up dialog */
    .backdrop {
      position: fixed; inset: 0;
      background: rgba(0, 0, 0, 0.55);
      display: flex; align-items: center; justify-content: center;
      z-index: 1000;
    }
    .dialog {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 20px 22px;
      max-width: 440px;
      width: 92%;
      box-sizing: border-box;
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
    }
    .dialog h3 { margin: 0 0 10px; font-size: 15px; color: var(--text); }
    .dialog p { margin: 0 0 12px; font-size: 13px; color: var(--muted); line-height: 1.45; }
    .dialog .row { display: flex; gap: 10px; justify-content: flex-end; margin-top: 16px; }
    .dialog button { margin-top: 0; }
    .dialog button.secondary { background: transparent; color: var(--muted); border: 1px solid var(--border); }
    .dialog .err { color: var(--err); font-size: 12px; margin-top: 8px; }
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){this.loading=!0;try{this.state=await D.sshKeys(),this.error=this.state.error??""}catch(Q){this.error=Q.message}finally{this.loading=!1}}val(Q){return this.renderRoot.querySelector(`#${Q}`)?.value??""}clear(Q){let Y=this.renderRoot.querySelector(`#${Q}`);if(Y)Y.value=""}addKey=async(Q)=>{if(Q.preventDefault(),this.adding)return;this.addError="",this.notice="";let Y=this.val("pubkey").trim();if(!Y){this.addError="Paste a public key.";return}this.adding=!0;try{this.state=await D.addSshKey(Y,this.val("comment").trim()),this.error=this.state.error??"",this.notice="Key added.",this.clear("pubkey"),this.clear("comment")}catch(K){this.addError=K.message||"failed"}finally{this.adding=!1}};askDelete(Q){this.pendingFp=Q,this.pwError="",queueMicrotask(()=>{this.renderRoot.querySelector("#delpw")?.focus()})}cancelDelete=()=>{if(this.deleting)return;this.pendingFp="",this.pwError=""};confirmDelete=async()=>{if(this.deleting)return;let Q=this.val("delpw");if(!Q){this.pwError="Password required.";return}this.deleting=!0,this.pwError="",this.notice="";try{if(!await D.verifyPassword(Q)){this.pwError="Password is wrong.";return}this.state=await D.removeSshKey(this.pendingFp),this.error=this.state.error??"",this.notice="Key removed.",this.pendingFp=""}catch(Y){this.pwError=Y.message||"failed"}finally{this.deleting=!1}};onDialogKey=(Q)=>{if(Q.key==="Enter")Q.preventDefault(),this.confirmDelete();else if(Q.key==="Escape")Q.preventDefault(),this.cancelDelete()};renderKey(Q){return q`
      <li class="key">
        <div class="keymeta">
          ${Q.comment?q`<div class="comment">${Q.comment}</div>`:q`<div class="comment none">(no comment)</div>`}
          <div class="fp">${Q.fingerprint}</div>
          <div class="added">Added ${Y4(Q.added_ms)}</div>
        </div>
        <button
          class="danger"
          type="button"
          @click=${()=>this.askDelete(Q.fingerprint)}
          ?disabled=${this.deleting}
        >
          Remove
        </button>
      </li>
    `}renderKeysPanel(){let Q=this.state?.keys??[],Y=this.state?.provider??"";return q`
      <div class="panel">
        <h2>SSH keys</h2>
        <p class="sub">
          Authorized keys for shell access${Y?q` · provider: ${Y}`:J}${this.state?.host_user?q` (${this.state.host_user})`:J}.
        </p>
        ${this.notice?q`<div class="banner ok">${this.notice}</div>`:J}
        ${this.error?q`<div class="banner err">⚠ ${this.error}</div>`:J}
        ${this.loading&&!this.state?q`<div class="loading">Loading…</div>`:Q.length===0?q`<div class="nudge">
                No SSH keys — add one below for shell access.
              </div>`:q`<ul class="keys">
                ${Q.map((K)=>this.renderKey(K))}
              </ul>`}

        <form class="addrow" @submit=${this.addKey}>
          <label for="pubkey">Public key</label>
          <textarea
            id="pubkey"
            placeholder="ssh-ed25519 AAAA… user@host"
            spellcheck="false"
            ?disabled=${this.adding}
          ></textarea>
          <label for="comment">Comment (optional)</label>
          <input id="comment" type="text" placeholder="laptop" ?disabled=${this.adding} />
          <button class="primary" type="submit" ?disabled=${this.adding}>
            ${this.adding?"…":"Add key"}
          </button>
          ${this.addError?q`<div class="banner err" style="margin-top:12px">⚠ ${this.addError}</div>`:J}
        </form>
      </div>
    `}renderDeleteDialog(){if(!this.pendingFp)return J;return q`
      <div class="backdrop" @click=${(Q)=>{if(Q.target===Q.currentTarget)this.cancelDelete()}}>
        <div class="dialog" role="dialog" aria-modal="true">
          <h3>Remove SSH key</h3>
          <p>
            Removing this key revokes its shell access. If it's the only key,
            you may lose console-less access to the box. Confirm with your
            password.
          </p>
          <input
            id="delpw"
            type="password"
            autocomplete="current-password"
            placeholder="Password"
            @keydown=${this.onDialogKey}
            ?disabled=${this.deleting}
          />
          ${this.pwError?q`<div class="err">${this.pwError}</div>`:J}
          <div class="row">
            <button class="secondary" type="button" @click=${this.cancelDelete} ?disabled=${this.deleting}>
              Cancel
            </button>
            <button class="danger" type="button" @click=${()=>void this.confirmDelete()} ?disabled=${this.deleting}>
              ${this.deleting?"Removing…":"Remove key"}
            </button>
          </div>
        </div>
      </div>
    `}render(){return q`
      ${this.renderKeysPanel()}
      ${this.renderDeleteDialog()}
    `}}customElements.define("security-view",DQ);class LQ extends ${static properties={items:{attribute:!1},route:{type:String},open:{type:Boolean}};constructor(){super();this.items=[],this.route="dashboard",this.open=!1}close=()=>{this.dispatchEvent(new CustomEvent("close",{bubbles:!0,composed:!0}))};static styles=L`
    :host { display: block; height: 100%; }
    nav {
      height: 100%;
      box-sizing: border-box;
      background: var(--surface);
      border-right: 1px solid var(--border);
      padding: 20px 12px;
    }
    .brand {
      font-weight: 800;
      letter-spacing: 0.06em;
      color: var(--accent);
      padding: 0 12px 20px;
      font-size: 16px;
    }
    a.item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-radius: 8px;
      color: var(--muted);
      text-decoration: none;
      font-size: 14px;
      margin-bottom: 2px;
    }
    a.item:hover { background: var(--bar-bg); color: var(--text); }
    a.item.active { background: color-mix(in srgb, var(--accent) 18%, transparent); color: var(--accent); }
    .ic { width: 18px; text-align: center; opacity: 0.8; }
    .scrim { display: none; }
    @media (max-width: 720px) {
      :host { height: auto; }
      nav {
        position: fixed;
        top: 0;
        left: 0;
        bottom: 0;
        width: 240px;
        z-index: 30;
        transform: translateX(-100%);
        transition: transform 0.2s ease;
        overflow-y: auto;
      }
      nav.open { transform: translateX(0); box-shadow: 4px 0 32px rgba(0, 0, 0, 0.5); }
      .scrim { display: block; position: fixed; inset: 0; background: rgba(0, 0, 0, 0.5); z-index: 20; }
    }
  `;render(){return q`
      <nav class=${this.open?"open":""}>
        <div class="brand">ECU CONSOLE</div>
        ${this.items.map((Q)=>q`<a
            class="item ${this.route===Q.id?"active":""}"
            href="#/${Q.id}"
            @click=${this.close}
          ><span class="ic">${Q.icon}</span>${Q.label}</a>`)}
      </nav>
      ${this.open?q`<div class="scrim" @click=${this.close}></div>`:J}
    `}}customElements.define("app-nav",LQ);var V3=[{id:"dashboard",label:"Dashboard",icon:"▮▮"},{id:"inverters",label:"Inverters",icon:"⌁"},{id:"alarms",label:"Alarms",icon:"!"},{id:"events",label:"Events",icon:"≣"},{id:"profiles",label:"Profiles",icon:"⛭"},{id:"settings",label:"Settings",icon:"⚙"},{id:"security",label:"Security",icon:"⚿"}];class FQ extends ${static properties={ready:{state:!0},authed:{state:!0},configured:{state:!0},route:{state:!0},fleet:{state:!0},system:{state:!0},names:{state:!0},customProfiles:{state:!0},navOpen:{state:!0}};closeSSE=null;sysTimer=null;settingsCache=null;constructor(){super();this.ready=!1,this.authed=!1,this.configured=!0,this.route="dashboard",this.fleet=null,this.system=null,this.names={},this.customProfiles={},this.navOpen=!1}static styles=L`
    :host { display: block; }
    .layout { display: grid; grid-template-columns: 220px 1fr; min-height: 100vh; }
    main { padding: 24px 28px; }
    .topbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 22px;
    }
    h1 { font-size: 20px; margin: 0; color: var(--text); }
    .right { display: flex; align-items: center; gap: 16px; }
    .conn { font-size: 12px; color: var(--muted); display: flex; align-items: center; gap: 6px; }
    .dot { width: 8px; height: 8px; border-radius: 50%; }
    .dot.on { background: var(--ok); box-shadow: 0 0 6px var(--ok); }
    .dot.off { background: var(--err); }
    button.logout {
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 8px;
      padding: 6px 12px;
      font-size: 13px;
      cursor: pointer;
    }
    button.logout:hover { color: var(--text); border-color: var(--muted); }
    .titlewrap { display: flex; align-items: center; gap: 12px; min-width: 0; }
    h1 { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    button.hamburger {
      display: none;
      background: transparent;
      border: 1px solid var(--border);
      color: var(--text);
      border-radius: 8px;
      padding: 5px 10px;
      font-size: 17px;
      line-height: 1;
      cursor: pointer;
    }
    @media (max-width: 720px) {
      .layout {
        grid-template-columns: 1fr;
        /* On mobile, app-nav is position:fixed → its grid row is empty.
           Without this, the two implicit rows stretch (align-content: normal
           ≈ stretch) and split min-height:100vh 50/50, pushing main into the
           vertical middle. Pin row 1 to content (0) and row 2 to 1fr so main
           top-aligns and fills the viewport. */
        grid-template-rows: auto 1fr;
      }
      button.hamburger { display: inline-flex; }
      main { padding: 18px 16px; }
    }
  `;connectedCallback(){super.connectedCallback(),window.addEventListener("hashchange",this.onHash),this.onHash(),this.init()}disconnectedCallback(){super.disconnectedCallback(),window.removeEventListener("hashchange",this.onHash),this.stopStreams()}onHash=()=>{let Q=(location.hash.replace(/^#\/?/,"")||"dashboard").split("/")[0];if(this.route=V3.some((Y)=>Y.id===Q)?Q:"dashboard",this.navOpen=!1,this.route==="dashboard"&&this.authed)this.fetchOverlays()};async init(){try{let Q=await D.authStatus();if(this.configured=Q.configured,this.authed=Q.authenticated,this.authed)this.startStreams()}catch{}finally{this.ready=!0}}onAuthed=async()=>{this.authed=!0,this.configured=!0,this.startStreams()};logout=async()=>{try{await D.logout()}catch{}this.authed=!1,this.stopStreams(),this.fleet=null,this.system=null};startStreams(){this.stopStreams(),this.closeSSE=d3((Y)=>{this.fleet=Y});let Q=()=>D.system().then((Y)=>this.system=Y).catch(()=>{});Q(),this.sysTimer=setInterval(Q,5000),this.fetchSettings(),this.fetchOverlays()}async fetchSettings(){try{let Q=await D.getSettings();if(Q.settings)this.settingsCache=Q.settings,this.names=Q.settings.inverter_names??{}}catch{}}async fetchOverlays(){try{let Q=await D.overlays(),Y={};for(let K of Q)for(let X of K.uids)Y[X]=K.id;this.customProfiles=Y}catch{}}onRename=async(Q)=>{let{uid:Y,name:K}=Q.detail,X=this.settingsCache??{ecu_id:"",mac:"",pan_override:"",zigbee_type:""},G={...X.inverter_names??{}};if(K.trim())G[Y]=K.trim();else delete G[Y];let z={...X,inverter_names:G};try{await D.saveSettings(z),this.settingsCache=z,this.names=G}catch{}};stopStreams(){if(this.closeSSE?.(),this.closeSSE=null,this.sysTimer)clearInterval(this.sysTimer);this.sysTimer=null}activeView(){switch(this.route){case"inverters":return q`<inverters-view
          .fleet=${this.fleet}
          .names=${this.names}
          @rename=${this.onRename}
        ></inverters-view>`;case"alarms":return q`<alarms-view .fleet=${this.fleet}></alarms-view>`;case"events":return q`<events-view></events-view>`;case"profiles":return q`<profiles-view></profiles-view>`;case"settings":return q`<settings-view></settings-view>`;case"security":return q`<security-view></security-view>`;default:return q`<dashboard-view
          .fleet=${this.fleet}
          .system=${this.system}
          .names=${this.names}
          .profiles=${this.customProfiles}
        ></dashboard-view>`}}render(){if(!this.ready)return J;if(!this.authed)return q`<login-view .configured=${this.configured} @authed=${this.onAuthed}></login-view>`;let Q=V3.find((K)=>K.id===this.route)?.label??"Dashboard",Y=this.system?.invdriver_connected??!1;return q`
      <div class="layout">
        <app-nav
          .items=${V3}
          .route=${this.route}
          .open=${this.navOpen}
          @close=${()=>this.navOpen=!1}
        ></app-nav>
        <main>
          <div class="topbar">
            <div class="titlewrap">
              <button class="hamburger" aria-label="Menu" aria-expanded=${this.navOpen} @click=${()=>this.navOpen=!this.navOpen}>☰</button>
              <h1>${Q}</h1>
            </div>
            <div class="right">
              <span class="conn">
                <span class="dot ${Y?"on":"off"}"></span>
                inv-driver ${Y?"connected":"down"}
              </span>
              <button class="logout" @click=${this.logout}>Sign out</button>
            </div>
          </div>
          ${this.activeView()}
        </main>
      </div>
    `}}customElements.define("ecu-app",FQ);
