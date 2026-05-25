var a=globalThis,GX=a.ShadowRoot&&(a.ShadyCSS===void 0||a.ShadyCSS.nativeShadow)&&"adoptedStyleSheets"in Document.prototype&&"replace"in CSSStyleSheet.prototype,HX=Symbol(),SX=new WeakMap;class JX{constructor(X,Z,K){if(this._$cssResult$=!0,K!==HX)throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");this.cssText=X,this._strings=Z}get styleSheet(){let X=this._styleSheet,Z=this._strings;if(GX&&X===void 0){let K=Z!==void 0&&Z.length===1;if(K)X=SX.get(Z);if(X===void 0){if((this._styleSheet=X=new CSSStyleSheet).replaceSync(this.cssText),K)SX.set(Z,X)}}return X}toString(){return this.cssText}}var RZ=(X)=>{if(X._$cssResult$===!0)return X.cssText;else if(typeof X==="number")return X;else throw Error(`Value passed to 'css' function must be a 'css' function result: ${X}. Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.`)},LZ=(X)=>new JX(typeof X==="string"?X:String(X),void 0,HX),J=(X,...Z)=>{let K=X.length===1?X[0]:Z.reduce((Y,Q,B)=>Y+RZ(Q)+X[B+1],X[0]);return new JX(K,X,HX)},PX=(X,Z)=>{if(GX)X.adoptedStyleSheets=Z.map((K)=>K instanceof CSSStyleSheet?K:K.styleSheet);else for(let K of Z){let Y=document.createElement("style"),Q=a.litNonce;if(Q!==void 0)Y.setAttribute("nonce",Q);Y.textContent=K.cssText,X.appendChild(Y)}},SZ=(X)=>{let Z="";for(let K of X.cssRules)Z+=K.cssText;return LZ(Z)},qX=GX?(X)=>X:(X)=>X instanceof CSSStyleSheet?SZ(X):X;var{is:PZ,defineProperty:yZ,getOwnPropertyDescriptor:yX,getOwnPropertyNames:xZ,getOwnPropertySymbols:bZ,getPrototypeOf:xX}=Object,EZ=!1,A=globalThis;if(EZ)A.customElements??=customElements;var I=!0,C,bX=A.trustedTypes,fZ=bX?bX.emptyScript:"",fX=I?A.reactiveElementPolyfillSupportDevMode:A.reactiveElementPolyfillSupport;if(I)A.litIssuedWarnings??=new Set,C=(X,Z)=>{if(Z+=` See https://lit.dev/msg/${X} for more information.`,!A.litIssuedWarnings.has(Z)&&!A.litIssuedWarnings.has(X))console.warn(Z),A.litIssuedWarnings.add(Z)},queueMicrotask(()=>{if(C("dev-mode","Lit is in dev mode. Not recommended for production!"),A.ShadyDOM?.inUse&&fX===void 0)C("polyfill-support-missing","Shadow DOM is being polyfilled via `ShadyDOM` but the `polyfill-support` module has not been loaded.")});var wZ=I?(X)=>{if(!A.emitLitDebugLogEvents)return;A.dispatchEvent(new CustomEvent("lit-debug",{detail:X}))}:void 0,w=(X,Z)=>X,FX={toAttribute(X,Z){switch(Z){case Boolean:X=X?fZ:null;break;case Object:case Array:X=X==null?X:JSON.stringify(X);break}return X},fromAttribute(X,Z){let K=X;switch(Z){case Boolean:K=X!==null;break;case Number:K=X===null?null:Number(X);break;case Object:case Array:try{K=JSON.parse(X)}catch(Y){K=null}break}return K}},wX=(X,Z)=>!PZ(X,Z),EX={attribute:!0,type:String,converter:FX,reflect:!1,useDefault:!1,hasChanged:wX};Symbol.metadata??=Symbol("metadata");A.litPropertyMetadata??=new WeakMap;class O extends HTMLElement{static addInitializer(X){this.__prepare(),(this._initializers??=[]).push(X)}static get observedAttributes(){return this.finalize(),this.__attributeToPropertyMap&&[...this.__attributeToPropertyMap.keys()]}static createProperty(X,Z=EX){if(Z.state)Z.attribute=!1;if(this.__prepare(),this.prototype.hasOwnProperty(X))Z=Object.create(Z),Z.wrapped=!0;if(this.elementProperties.set(X,Z),!Z.noAccessor){let K=I?Symbol.for(`${String(X)} (@property() cache)`):Symbol(),Y=this.getPropertyDescriptor(X,K,Z);if(Y!==void 0)yZ(this.prototype,X,Y)}}static getPropertyDescriptor(X,Z,K){let{get:Y,set:Q}=yX(this.prototype,X)??{get(){return this[Z]},set(B){this[Z]=B}};if(I&&Y==null){if("value"in(yX(this.prototype,X)??{}))throw Error(`Field ${JSON.stringify(String(X))} on ${this.name} was declared as a reactive property but it's actually declared as a value on the prototype. Usually this is due to using @property or @state on a method.`);C("reactive-property-without-getter",`Field ${JSON.stringify(String(X))} on ${this.name} was declared as a reactive property but it does not have a getter. This will be an error in a future version of Lit.`)}return{get:Y,set(B){let j=Y?.call(this);Q?.call(this,B),this.requestUpdate(X,j,K)},configurable:!0,enumerable:!0}}static getPropertyOptions(X){return this.elementProperties.get(X)??EX}static __prepare(){if(this.hasOwnProperty(w("elementProperties",this)))return;let X=xX(this);if(X.finalize(),X._initializers!==void 0)this._initializers=[...X._initializers];this.elementProperties=new Map(X.elementProperties)}static finalize(){if(this.hasOwnProperty(w("finalized",this)))return;if(this.finalized=!0,this.__prepare(),this.hasOwnProperty(w("properties",this))){let Z=this.properties,K=[...xZ(Z),...bZ(Z)];for(let Y of K)this.createProperty(Y,Z[Y])}let X=this[Symbol.metadata];if(X!==null){let Z=litPropertyMetadata.get(X);if(Z!==void 0)for(let[K,Y]of Z)this.elementProperties.set(K,Y)}this.__attributeToPropertyMap=new Map;for(let[Z,K]of this.elementProperties){let Y=this.__attributeNameForProperty(Z,K);if(Y!==void 0)this.__attributeToPropertyMap.set(Y,Z)}if(this.elementStyles=this.finalizeStyles(this.styles),I){if(this.hasOwnProperty("createProperty"))C("no-override-create-property","Overriding ReactiveElement.createProperty() is deprecated. The override will not be called with standard decorators");if(this.hasOwnProperty("getPropertyDescriptor"))C("no-override-get-property-descriptor","Overriding ReactiveElement.getPropertyDescriptor() is deprecated. The override will not be called with standard decorators")}}static finalizeStyles(X){let Z=[];if(Array.isArray(X)){let K=new Set(X.flat(1/0).reverse());for(let Y of K)Z.unshift(qX(Y))}else if(X!==void 0)Z.push(qX(X));return Z}static __attributeNameForProperty(X,Z){let K=Z.attribute;return K===!1?void 0:typeof K==="string"?K:typeof X==="string"?X.toLowerCase():void 0}constructor(){super();this.__instanceProperties=void 0,this.isUpdatePending=!1,this.hasUpdated=!1,this.__reflectingProperty=null,this.__initialize()}__initialize(){this.__updatePromise=new Promise((X)=>this.enableUpdating=X),this._$changedProperties=new Map,this.__saveInstanceProperties(),this.requestUpdate(),this.constructor._initializers?.forEach((X)=>X(this))}addController(X){if((this.__controllers??=new Set).add(X),this.renderRoot!==void 0&&this.isConnected)X.hostConnected?.()}removeController(X){this.__controllers?.delete(X)}__saveInstanceProperties(){let X=new Map,Z=this.constructor.elementProperties;for(let K of Z.keys())if(this.hasOwnProperty(K))X.set(K,this[K]),delete this[K];if(X.size>0)this.__instanceProperties=X}createRenderRoot(){let X=this.shadowRoot??this.attachShadow(this.constructor.shadowRootOptions);return PX(X,this.constructor.elementStyles),X}connectedCallback(){this.renderRoot??=this.createRenderRoot(),this.enableUpdating(!0),this.__controllers?.forEach((X)=>X.hostConnected?.())}enableUpdating(X){}disconnectedCallback(){this.__controllers?.forEach((X)=>X.hostDisconnected?.())}attributeChangedCallback(X,Z,K){this._$attributeToProperty(X,K)}__propertyToAttribute(X,Z){let Y=this.constructor.elementProperties.get(X),Q=this.constructor.__attributeNameForProperty(X,Y);if(Q!==void 0&&Y.reflect===!0){let j=(Y.converter?.toAttribute!==void 0?Y.converter:FX).toAttribute(Z,Y.type);if(I&&this.constructor.enabledWarnings.includes("migration")&&j===void 0)C("undefined-attribute-value",`The attribute value for the ${X} property is undefined on element ${this.localName}. The attribute will be removed, but in the previous version of \`ReactiveElement\`, the attribute would not have changed.`);if(this.__reflectingProperty=X,j==null)this.removeAttribute(Q);else this.setAttribute(Q,j);this.__reflectingProperty=null}}_$attributeToProperty(X,Z){let K=this.constructor,Y=K.__attributeToPropertyMap.get(X);if(Y!==void 0&&this.__reflectingProperty!==Y){let Q=K.getPropertyOptions(Y),B=typeof Q.converter==="function"?{fromAttribute:Q.converter}:Q.converter?.fromAttribute!==void 0?Q.converter:FX;this.__reflectingProperty=Y;let j=B.fromAttribute(Z,Q.type);this[Y]=j??this.__defaultValues?.get(Y)??j,this.__reflectingProperty=null}}requestUpdate(X,Z,K,Y=!1,Q){if(X!==void 0){if(I&&X instanceof Event)C("","The requestUpdate() method was called with an Event as the property name. This is probably a mistake caused by binding this.requestUpdate as an event listener. Instead bind a function that will call it with no arguments: () => this.requestUpdate()");let B=this.constructor;if(Y===!1)Q=this[X];if(K??=B.getPropertyOptions(X),(K.hasChanged??wX)(Q,Z)||K.useDefault&&K.reflect&&Q===this.__defaultValues?.get(X)&&!this.hasAttribute(B.__attributeNameForProperty(X,K)))this._$changeProperty(X,Z,K);else return}if(this.isUpdatePending===!1)this.__updatePromise=this.__enqueueUpdate()}_$changeProperty(X,Z,{useDefault:K,reflect:Y,wrapped:Q},B){if(K&&!(this.__defaultValues??=new Map).has(X)){if(this.__defaultValues.set(X,B??Z??this[X]),Q!==!0||B!==void 0)return}if(!this._$changedProperties.has(X)){if(!this.hasUpdated&&!K)Z=void 0;this._$changedProperties.set(X,Z)}if(Y===!0&&this.__reflectingProperty!==X)(this.__reflectingProperties??=new Set).add(X)}async __enqueueUpdate(){this.isUpdatePending=!0;try{await this.__updatePromise}catch(Z){Promise.reject(Z)}let X=this.scheduleUpdate();if(X!=null)await X;return!this.isUpdatePending}scheduleUpdate(){let X=this.performUpdate();if(I&&this.constructor.enabledWarnings.includes("async-perform-update")&&typeof X?.then==="function")C("async-perform-update",`Element ${this.localName} returned a Promise from performUpdate(). This behavior is deprecated and will be removed in a future version of ReactiveElement.`);return X}performUpdate(){if(!this.isUpdatePending)return;if(wZ?.({kind:"update"}),!this.hasUpdated){if(this.renderRoot??=this.createRenderRoot(),I){let Q=[...this.constructor.elementProperties.keys()].filter((B)=>this.hasOwnProperty(B)&&(B in xX(this)));if(Q.length)throw Error(`The following properties on element ${this.localName} will not trigger updates as expected because they are set using class fields: ${Q.join(", ")}. Native class fields and some compiled output will overwrite accessors used for detecting changes. See https://lit.dev/msg/class-field-shadowing for more information.`)}if(this.__instanceProperties){for(let[Y,Q]of this.__instanceProperties)this[Y]=Q;this.__instanceProperties=void 0}let K=this.constructor.elementProperties;if(K.size>0)for(let[Y,Q]of K){let{wrapped:B}=Q,j=this[Y];if(B===!0&&!this._$changedProperties.has(Y)&&j!==void 0)this._$changeProperty(Y,void 0,Q,j)}}let X=!1,Z=this._$changedProperties;try{if(X=this.shouldUpdate(Z),X)this.willUpdate(Z),this.__controllers?.forEach((K)=>K.hostUpdate?.()),this.update(Z);else this.__markUpdated()}catch(K){throw X=!1,this.__markUpdated(),K}if(X)this._$didUpdate(Z)}willUpdate(X){}_$didUpdate(X){if(this.__controllers?.forEach((Z)=>Z.hostUpdated?.()),!this.hasUpdated)this.hasUpdated=!0,this.firstUpdated(X);if(this.updated(X),I&&this.isUpdatePending&&this.constructor.enabledWarnings.includes("change-in-update"))C("change-in-update",`Element ${this.localName} scheduled an update (generally because a property was set) after an update completed, causing a new update to be scheduled. This is inefficient and should be avoided unless the next update can only be scheduled as a side effect of the previous update.`)}__markUpdated(){this._$changedProperties=new Map,this.isUpdatePending=!1}get updateComplete(){return this.getUpdateComplete()}getUpdateComplete(){return this.__updatePromise}shouldUpdate(X){return!0}update(X){this.__reflectingProperties&&=this.__reflectingProperties.forEach((Z)=>this.__propertyToAttribute(Z,this[Z])),this.__markUpdated()}updated(X){}firstUpdated(X){}}O.elementStyles=[];O.shadowRootOptions={mode:"open"};O[w("elementProperties",O)]=new Map;O[w("finalized",O)]=new Map;fX?.({ReactiveElement:O});if(I){O.enabledWarnings=["change-in-update","async-perform-update"];let X=function(Z){if(!Z.hasOwnProperty(w("enabledWarnings",Z)))Z.enabledWarnings=Z.enabledWarnings.slice()};O.enableWarning=function(Z){if(X(this),!this.enabledWarnings.includes(Z))this.enabledWarnings.push(Z)},O.disableWarning=function(Z){X(this);let K=this.enabledWarnings.indexOf(Z);if(K>=0)this.enabledWarnings.splice(K,1)}}(A.reactiveElementVersions??=[]).push("2.1.2");if(I&&A.reactiveElementVersions.length>1)queueMicrotask(()=>{C("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var T=globalThis,F=(X)=>{if(!T.emitLitDebugLogEvents)return;T.dispatchEvent(new CustomEvent("lit-debug",{detail:X}))},hZ=0,u;T.litIssuedWarnings??=new Set,u=(X,Z)=>{if(Z+=X?` See https://lit.dev/msg/${X} for more information.`:"",!T.litIssuedWarnings.has(Z)&&!T.litIssuedWarnings.has(X))console.warn(Z),T.litIssuedWarnings.add(Z)},queueMicrotask(()=>{u("dev-mode","Lit is in dev mode. Not recommended for production!")});var D=T.ShadyDOM?.inUse&&T.ShadyDOM?.noPatch===!0?T.ShadyDOM.wrap:(X)=>X,n=T.trustedTypes,hX=n?n.createPolicy("lit-html",{createHTML:(X)=>X}):void 0,gZ=(X)=>X,ZX=(X,Z,K)=>gZ,cZ=(X)=>{if(f!==ZX)throw Error("Attempted to overwrite existing lit-html security policy. setSanitizeDOMValueFactory should be called at most once.");f=X},dZ=()=>{f=ZX},_X=(X,Z,K)=>{return f(X,Z,K)},pX="$lit$",R=`lit$${Math.random().toFixed(9).slice(2)}$`,oX="?"+R,uZ=`<${oX}>`,b=document,m=()=>b.createComment(""),v=(X)=>X===null||typeof X!="object"&&typeof X!="function",AX=Array.isArray,mZ=(X)=>AX(X)||typeof X?.[Symbol.iterator]==="function",UX=`[ 	
\f\r]`,vZ=`[^ 	
\f\r"'\`<>=]`,pZ=`[^\\s"'>=/]`,c=/<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g,gX=1,kX=2,oZ=3,cX=/-->/g,dX=/>/g,S=new RegExp(`>|${UX}(?:(${pZ}+)(${UX}*=${UX}*(?:${vZ}|("|')|))|$)`,"g"),lZ=0,uX=1,sZ=2,mX=3,MX=/'/g,WX=/"/g,lX=/^(?:script|style|textarea|title)$/i,iZ=1,t=2,e=3,IX=1,XX=2,rZ=3,aZ=4,nZ=5,OX=6,tZ=7,TX=(X)=>(Z,...K)=>{if(Z.some((Y)=>Y===void 0))console.warn(`Some template strings are undefined.
This is probably caused by illegal octal escape sequences.`);if(K.some((Y)=>Y?._$litStatic$))u("",`Static values 'literal' or 'unsafeStatic' cannot be used as values to non-static templates.
Please use the static 'html' tag function. See https://lit.dev/docs/templates/expressions/#static-expressions`);return{["_$litType$"]:X,strings:Z,values:K}},$=TX(iZ),KX=TX(t),J5=TX(e),E=Symbol.for("lit-noChange"),z=Symbol.for("lit-nothing"),vX=new WeakMap,x=b.createTreeWalker(b,129),f=ZX;function sX(X,Z){if(!AX(X)||!X.hasOwnProperty("raw")){let K="invalid template strings array";throw K=`
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
`),Error(K)}return hX!==void 0?hX.createHTML(Z):Z}var eZ=(X,Z)=>{let K=X.length-1,Y=[],Q=Z===t?"<svg>":Z===e?"<math>":"",B,j=c;for(let U=0;U<K;U++){let _=X[U],q=-1,k,N=0,W;while(N<_.length){if(j.lastIndex=N,W=j.exec(_),W===null)break;if(N=j.lastIndex,j===c){if(W[gX]==="!--")j=cX;else if(W[gX]!==void 0)j=dX;else if(W[kX]!==void 0){if(lX.test(W[kX]))B=new RegExp(`</${W[kX]}`,"g");j=S}else if(W[oZ]!==void 0)throw Error("Bindings in tag names are not supported. Please use static templates instead. See https://lit.dev/docs/templates/expressions/#static-expressions")}else if(j===S)if(W[lZ]===">")j=B??c,q=-1;else if(W[uX]===void 0)q=-2;else q=j.lastIndex-W[sZ].length,k=W[uX],j=W[mX]===void 0?S:W[mX]==='"'?WX:MX;else if(j===WX||j===MX)j=S;else if(j===cX||j===dX)j=c;else j=S,B=void 0}console.assert(q===-1||j===S||j===MX||j===WX,"unexpected parse state B");let y=j===S&&X[U+1].startsWith("/>")?" ":"";Q+=j===c?_+uZ:q>=0?(Y.push(k),_.slice(0,q)+pX+_.slice(q))+R+y:_+R+(q===-2?U:y)}let H=Q+(X[K]||"<?>")+(Z===t?"</svg>":Z===e?"</math>":"");return[sX(X,H),Y]};class p{constructor({strings:X,["_$litType$"]:Z},K){this.parts=[];let Y,Q=0,B=0,j=X.length-1,H=this.parts,[U,_]=eZ(X,Z);if(this.el=p.createElement(U,K),x.currentNode=this.el.content,Z===t||Z===e){let q=this.el.content.firstChild;q.replaceWith(...q.childNodes)}while((Y=x.nextNode())!==null&&H.length<j){if(Y.nodeType===1){{let q=Y.localName;if(/^(?:textarea|template)$/i.test(q)&&Y.innerHTML.includes(R)){let k=`Expressions are not supported inside \`${q}\` elements. See https://lit.dev/msg/expression-in-${q} for more information.`;if(q==="template")throw Error(k);else u("",k)}}if(Y.hasAttributes()){for(let q of Y.getAttributeNames())if(q.endsWith(pX)){let k=_[B++],W=Y.getAttribute(q).split(R),y=/([.?@])?(.*)/.exec(k);H.push({type:IX,index:Q,name:y[2],strings:W,ctor:y[1]==="."?rX:y[1]==="?"?aX:y[1]==="@"?nX:l}),Y.removeAttribute(q)}else if(q.startsWith(R))H.push({type:OX,index:Q}),Y.removeAttribute(q)}if(lX.test(Y.tagName)){let q=Y.textContent.split(R),k=q.length-1;if(k>0){Y.textContent=n?n.emptyScript:"";for(let N=0;N<k;N++)Y.append(q[N],m()),x.nextNode(),H.push({type:XX,index:++Q});Y.append(q[k],m())}}}else if(Y.nodeType===8)if(Y.data===oX)H.push({type:XX,index:Q});else{let k=-1;while((k=Y.data.indexOf(R,k+1))!==-1)H.push({type:tZ,index:Q}),k+=R.length-1}Q++}if(_.length!==B)throw Error('Detected duplicate attribute bindings. This occurs if your template has duplicate attributes on an element tag. For example "<input ?disabled=${true} ?disabled=${false}>" contains a duplicate "disabled" attribute. The error was detected in the following template: \n`'+X.join("${...}")+"`");F&&F({kind:"template prep",template:this,clonableTemplate:this.el,parts:this.parts,strings:X})}static createElement(X,Z){let K=b.createElement("template");return K.innerHTML=X,K}}function h(X,Z,K=X,Y){if(Z===E)return Z;let Q=Y!==void 0?K.__directives?.[Y]:K.__directive,B=v(Z)?void 0:Z._$litDirective$;if(Q?.constructor!==B){if(Q?._$notifyDirectiveConnectionChanged?.(!1),B===void 0)Q=void 0;else Q=new B(X),Q._$initialize(X,K,Y);if(Y!==void 0)(K.__directives??=[])[Y]=Q;else K.__directive=Q}if(Q!==void 0)Z=h(X,Q._$resolve(X,Z.values),Q,Y);return Z}class iX{constructor(X,Z){this._$parts=[],this._$disconnectableChildren=void 0,this._$template=X,this._$parent=Z}get parentNode(){return this._$parent.parentNode}get _$isConnected(){return this._$parent._$isConnected}_clone(X){let{el:{content:Z},parts:K}=this._$template,Y=(X?.creationScope??b).importNode(Z,!0);x.currentNode=Y;let Q=x.nextNode(),B=0,j=0,H=K[0];while(H!==void 0){if(B===H.index){let U;if(H.type===XX)U=new o(Q,Q.nextSibling,this,X);else if(H.type===IX)U=new H.ctor(Q,H.name,H.strings,this,X);else if(H.type===OX)U=new tX(Q,this,X);this._$parts.push(U),H=K[++j]}if(B!==H?.index)Q=x.nextNode(),B++}return x.currentNode=b,Y}_update(X){let Z=0;for(let K of this._$parts){if(K!==void 0)if(F&&F({kind:"set part",part:K,value:X[Z],valueIndex:Z,values:X,templateInstance:this}),K.strings!==void 0)K._$setValue(X,K,Z),Z+=K.strings.length-2;else K._$setValue(X[Z]);Z++}}}class o{get _$isConnected(){return this._$parent?._$isConnected??this.__isConnected}constructor(X,Z,K,Y){this.type=XX,this._$committedValue=z,this._$disconnectableChildren=void 0,this._$startNode=X,this._$endNode=Z,this._$parent=K,this.options=Y,this.__isConnected=Y?.isConnected??!0,this._textSanitizer=void 0}get parentNode(){let X=D(this._$startNode).parentNode,Z=this._$parent;if(Z!==void 0&&X?.nodeType===11)X=Z.parentNode;return X}get startNode(){return this._$startNode}get endNode(){return this._$endNode}_$setValue(X,Z=this){if(this.parentNode===null)throw Error("This `ChildPart` has no `parentNode` and therefore cannot accept a value. This likely means the element containing the part was manipulated in an unsupported way outside of Lit's control such that the part's marker nodes were ejected from DOM. For example, setting the element's `innerHTML` or `textContent` can do this.");if(X=h(this,X,Z),v(X)){if(X===z||X==null||X===""){if(this._$committedValue!==z)F&&F({kind:"commit nothing to child",start:this._$startNode,end:this._$endNode,parent:this._$parent,options:this.options}),this._$clear();this._$committedValue=z}else if(X!==this._$committedValue&&X!==E)this._commitText(X)}else if(X._$litType$!==void 0)this._commitTemplateResult(X);else if(X.nodeType!==void 0){if(this.options?.host===X){this._commitText("[probable mistake: rendered a template's host in itself (commonly caused by writing ${this} in a template]"),console.warn("Attempted to render the template host",X,"inside itself. This is almost always a mistake, and in dev mode ","we render some warning text. In production however, we'll ","render it, which will usually result in an error, and sometimes ","in the element disappearing from the DOM.");return}this._commitNode(X)}else if(mZ(X))this._commitIterable(X);else this._commitText(X)}_insert(X){return D(D(this._$startNode).parentNode).insertBefore(X,this._$endNode)}_commitNode(X){if(this._$committedValue!==X){if(this._$clear(),f!==ZX){let Z=this._$startNode.parentNode?.nodeName;if(Z==="STYLE"||Z==="SCRIPT"){let K="Forbidden";if(Z==="STYLE")K="Lit does not support binding inside style nodes. This is a security risk, as style injection attacks can exfiltrate data and spoof UIs. Consider instead using css`...` literals to compose styles, and do dynamic styling with css custom properties, ::parts, <slot>s, and by mutating the DOM rather than stylesheets.";else K="Lit does not support binding inside script nodes. This is a security risk, as it could allow arbitrary code execution.";throw Error(K)}}F&&F({kind:"commit node",start:this._$startNode,parent:this._$parent,value:X,options:this.options}),this._$committedValue=this._insert(X)}}_commitText(X){if(this._$committedValue!==z&&v(this._$committedValue)){let Z=D(this._$startNode).nextSibling;if(this._textSanitizer===void 0)this._textSanitizer=_X(Z,"data","property");X=this._textSanitizer(X),F&&F({kind:"commit text",node:Z,value:X,options:this.options}),Z.data=X}else{let Z=b.createTextNode("");if(this._commitNode(Z),this._textSanitizer===void 0)this._textSanitizer=_X(Z,"data","property");X=this._textSanitizer(X),F&&F({kind:"commit text",node:Z,value:X,options:this.options}),Z.data=X}this._$committedValue=X}_commitTemplateResult(X){let{values:Z,["_$litType$"]:K}=X,Y=typeof K==="number"?this._$getTemplate(X):(K.el===void 0&&(K.el=p.createElement(sX(K.h,K.h[0]),this.options)),K);if(this._$committedValue?._$template===Y)F&&F({kind:"template updating",template:Y,instance:this._$committedValue,parts:this._$committedValue._$parts,options:this.options,values:Z}),this._$committedValue._update(Z);else{let Q=new iX(Y,this),B=Q._clone(this.options);F&&F({kind:"template instantiated",template:Y,instance:Q,parts:Q._$parts,options:this.options,fragment:B,values:Z}),Q._update(Z),F&&F({kind:"template instantiated and updated",template:Y,instance:Q,parts:Q._$parts,options:this.options,fragment:B,values:Z}),this._commitNode(B),this._$committedValue=Q}}_$getTemplate(X){let Z=vX.get(X.strings);if(Z===void 0)vX.set(X.strings,Z=new p(X));return Z}_commitIterable(X){if(!AX(this._$committedValue))this._$committedValue=[],this._$clear();let Z=this._$committedValue,K=0,Y;for(let Q of X){if(K===Z.length)Z.push(Y=new o(this._insert(m()),this._insert(m()),this,this.options));else Y=Z[K];Y._$setValue(Q),K++}if(K<Z.length)this._$clear(Y&&D(Y._$endNode).nextSibling,K),Z.length=K}_$clear(X=D(this._$startNode).nextSibling,Z){this._$notifyConnectionChanged?.(!1,!0,Z);while(X!==this._$endNode){let K=D(X).nextSibling;D(X).remove(),X=K}}setConnected(X){if(this._$parent===void 0)this.__isConnected=X,this._$notifyConnectionChanged?.(X);else throw Error("part.setConnected() may only be called on a RootPart returned from render().")}}class l{get tagName(){return this.element.tagName}get _$isConnected(){return this._$parent._$isConnected}constructor(X,Z,K,Y,Q){if(this.type=IX,this._$committedValue=z,this._$disconnectableChildren=void 0,this.element=X,this.name=Z,this._$parent=Y,this.options=Q,K.length>2||K[0]!==""||K[1]!=="")this._$committedValue=Array(K.length-1).fill(new String),this.strings=K;else this._$committedValue=z;this._sanitizer=void 0}_$setValue(X,Z=this,K,Y){let Q=this.strings,B=!1;if(Q===void 0){if(X=h(this,X,Z,0),B=!v(X)||X!==this._$committedValue&&X!==E,B)this._$committedValue=X}else{let j=X;X=Q[0];let H,U;for(H=0;H<Q.length-1;H++){if(U=h(this,j[K+H],Z,H),U===E)U=this._$committedValue[H];if(B||=!v(U)||U!==this._$committedValue[H],U===z)X=z;else if(X!==z)X+=(U??"")+Q[H+1];this._$committedValue[H]=U}}if(B&&!Y)this._commitValue(X)}_commitValue(X){if(X===z)D(this.element).removeAttribute(this.name);else{if(this._sanitizer===void 0)this._sanitizer=f(this.element,this.name,"attribute");X=this._sanitizer(X??""),F&&F({kind:"commit attribute",element:this.element,name:this.name,value:X,options:this.options}),D(this.element).setAttribute(this.name,X??"")}}}class rX extends l{constructor(){super(...arguments);this.type=rZ}_commitValue(X){if(this._sanitizer===void 0)this._sanitizer=f(this.element,this.name,"property");X=this._sanitizer(X),F&&F({kind:"commit property",element:this.element,name:this.name,value:X,options:this.options}),this.element[this.name]=X===z?void 0:X}}class aX extends l{constructor(){super(...arguments);this.type=aZ}_commitValue(X){F&&F({kind:"commit boolean attribute",element:this.element,name:this.name,value:!!(X&&X!==z),options:this.options}),D(this.element).toggleAttribute(this.name,!!X&&X!==z)}}class nX extends l{constructor(X,Z,K,Y,Q){super(X,Z,K,Y,Q);if(this.type=nZ,this.strings!==void 0)throw Error(`A \`<${X.localName}>\` has a \`@${Z}=...\` listener with invalid content. Event listeners in templates must have exactly one expression and no surrounding text.`)}_$setValue(X,Z=this){if(X=h(this,X,Z,0)??z,X===E)return;let K=this._$committedValue,Y=X===z&&K!==z||X.capture!==K.capture||X.once!==K.once||X.passive!==K.passive,Q=X!==z&&(K===z||Y);if(F&&F({kind:"commit event listener",element:this.element,name:this.name,value:X,options:this.options,removeListener:Y,addListener:Q,oldListener:K}),Y)this.element.removeEventListener(this.name,this,K);if(Q)this.element.addEventListener(this.name,this,X);this._$committedValue=X}handleEvent(X){if(typeof this._$committedValue==="function")this._$committedValue.call(this.options?.host??this.element,X);else this._$committedValue.handleEvent(X)}}class tX{constructor(X,Z,K){this.element=X,this.type=OX,this._$disconnectableChildren=void 0,this._$parent=Z,this.options=K}get _$isConnected(){return this._$parent._$isConnected}_$setValue(X){F&&F({kind:"commit to element binding",element:this.element,value:X,options:this.options}),h(this,X)}}var X5=T.litHtmlPolyfillSupportDevMode;X5?.(p,o);(T.litHtmlVersions??=[]).push("3.3.3");if(T.litHtmlVersions.length>1)queueMicrotask(()=>{u("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var d=(X,Z,K)=>{if(Z==null)throw TypeError(`The container to render into may not be ${Z}`);let Y=hZ++,Q=K?.renderBefore??Z,B=Q._$litPart$;if(F&&F({kind:"begin render",id:Y,value:X,container:Z,options:K,part:B}),B===void 0){let j=K?.renderBefore??null;Q._$litPart$=B=new o(Z.insertBefore(m(),j),j,void 0,K??{})}return B._$setValue(X),F&&F({kind:"end render",id:Y,value:X,container:Z,options:K,part:B}),B};d.setSanitizer=cZ,d.createSanitizer=_X,d._testOnlyClearSanitizerFactoryDoNotCallOrElse=dZ;var Z5=(X,Z)=>X,VX=!0,L=globalThis,eX;if(VX)L.litIssuedWarnings??=new Set,eX=(X,Z)=>{if(Z+=` See https://lit.dev/msg/${X} for more information.`,!L.litIssuedWarnings.has(Z)&&!L.litIssuedWarnings.has(X))console.warn(Z),L.litIssuedWarnings.add(Z)};class G extends O{constructor(){super(...arguments);this.renderOptions={host:this},this.__childPart=void 0}createRenderRoot(){let X=super.createRenderRoot();return this.renderOptions.renderBefore??=X.firstChild,X}update(X){let Z=this.render();if(!this.hasUpdated)this.renderOptions.isConnected=this.isConnected;super.update(X),this.__childPart=d(Z,this.renderRoot,this.renderOptions)}connectedCallback(){super.connectedCallback(),this.__childPart?.setConnected(!0)}disconnectedCallback(){super.disconnectedCallback(),this.__childPart?.setConnected(!1)}render(){return E}}G._$litElement$=!0;G[Z5("finalized",G)]=!0;L.litElementHydrateSupport?.({LitElement:G});var K5=VX?L.litElementPolyfillSupportDevMode:L.litElementPolyfillSupport;K5?.({LitElement:G});(L.litElementVersions??=[]).push("4.2.2");if(VX&&L.litElementVersions.length>1)queueMicrotask(()=>{eX("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});async function P(X){let Z=await fetch(X,{credentials:"same-origin"});if(!Z.ok)throw Error(`${X}: ${Z.status}`);return await Z.json()}async function YX(X,Z){let K=await fetch(X,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Z)});if(!K.ok){let Y=await K.text();throw Error(Y.trim()||`${X}: ${K.status}`)}}async function XZ(X,Z){let K=await fetch(X,{method:"PUT",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Z)});if(!K.ok){let Y=await K.text();throw Error(Y.trim()||`${X}: ${K.status}`)}return await K.json()}async function Y5(X,Z){let K=await fetch(X,{method:"DELETE",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Z)});if(!K.ok){let Y=await K.text();throw Error(Y.trim()||`${X}: ${K.status}`)}return await K.json()}var M={authStatus:()=>P("/api/auth/status"),setup:(X)=>YX("/api/auth/setup",{password:X}),login:(X)=>YX("/api/auth/login",{password:X}),logout:()=>YX("/api/auth/logout",{}),fleet:()=>P("/api/fleet"),system:()=>P("/api/system"),history:()=>P("/api/history"),events:(X={})=>{let Z=new URLSearchParams;if(X.since_ms)Z.set("since_ms",String(X.since_ms));if(X.kind)Z.set("kind",X.kind);if(X.severity)Z.set("severity",X.severity);if(X.inverter_uid)Z.set("inverter_uid",X.inverter_uid);if(X.limit)Z.set("limit",String(X.limit));let K=Z.toString();return P("/api/events"+(K?`?${K}`:""))},getSettings:async()=>{let X=await P("/api/settings");if(X.error)return{error:X.error};return{settings:{ecu_id:X.ecu_id,mac:X.mac,pan_override:X.pan_override,zigbee_type:X.zigbee_type,inverter_names:X.inverter_names??{}}}},saveSettings:(X)=>XZ("/api/settings",X),profiles:()=>P("/api/profiles"),overlays:()=>P("/api/overlays"),selectBase:(X)=>YX("/api/profiles/base",{id:X}),saveOverlay:(X)=>XZ("/api/profiles/overlay",X),deleteOverlay:(X,Z)=>Y5("/api/profiles/overlay",{id:X,uids:Z})};function ZZ(X,Z){let K=new EventSource("/api/stream");return K.addEventListener("fleet",(Y)=>{try{X(JSON.parse(Y.data))}catch{}}),K.onerror=()=>Z?.(),()=>K.close()}class KZ extends G{static properties={configured:{type:Boolean},error:{state:!0},busy:{state:!0}};constructor(){super();this.configured=!0,this.error="",this.busy=!1}static styles=J`
    :host {
      display: grid;
      place-items: center;
      min-height: 100vh;
    }
    .box {
      width: 320px;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 28px;
    }
    h1 { font-size: 20px; margin: 0 0 4px; color: var(--text); }
    p { color: var(--muted); font-size: 13px; margin: 0 0 18px; }
    label { display: block; font-size: 12px; color: var(--muted); margin-bottom: 6px; }
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
    button {
      width: 100%;
      margin-top: 16px;
      padding: 10px;
      background: var(--accent);
      color: #04222b;
      border: none;
      border-radius: 8px;
      font-weight: 700;
      cursor: pointer;
    }
    button:disabled { opacity: 0.6; cursor: default; }
    .err { color: var(--err); font-size: 13px; margin-top: 12px; min-height: 16px; }
    .brand { color: var(--accent); font-weight: 700; letter-spacing: 0.04em; }
  `;async submit(X){X.preventDefault();let K=this.renderRoot.querySelector("input")?.value??"";this.busy=!0,this.error="";try{if(this.configured)await M.login(K);else await M.setup(K);this.dispatchEvent(new CustomEvent("authed",{bubbles:!0,composed:!0}))}catch(Y){this.error=Y.message||"failed"}finally{this.busy=!1}}render(){return $`
      <form class="box" @submit=${this.submit}>
        <h1><span class="brand">ECU</span> Console</h1>
        <p>
          ${this.configured?"Enter the operator password.":"First run — choose an operator password (min 8 characters)."}
        </p>
        <label for="pw">Password</label>
        <input
          id="pw"
          type="password"
          autocomplete=${this.configured?"current-password":"new-password"}
          ?disabled=${this.busy}
        />
        <button type="submit" ?disabled=${this.busy}>
          ${this.busy?"…":this.configured?"Sign in":"Set password"}
        </button>
        <div class="err">${this.error}</div>
      </form>
    `}}customElements.define("login-view",KZ);function V(X){if(!Number.isFinite(X))return"—";if(Math.abs(X)>=1000)return`${(X/1000).toFixed(2)} kW`;return`${Math.round(X)} W`}function s(X){if(!Number.isFinite(X))return"—";let Z=Math.abs(X);if(Z>=1e6)return`${(X/1e6).toFixed(2)} MWh`;if(Z>=1000)return`${(X/1000).toFixed(2)} kWh`;return`${Math.round(X)} Wh`}function g(X){return Number.isFinite(X)?`${X.toFixed(0)}%`:"—"}function i(X){return X>0?`${X.toFixed(1)} V`:"—"}function QX(X){return X>0?`${X.toFixed(2)} Hz`:"—"}function YZ(X){return Number.isFinite(X)?`${X.toFixed(2)} A`:"—"}function $X(X){if(!(X>0))return"idle";if(X<40)return"low";if(X<85)return"mid";return"high"}function QZ(X){if(!Number.isFinite(X)||X<0)return"—";if(X<60)return`${Math.round(X)}s ago`;if(X<3600)return`${Math.round(X/60)}m ago`;return`${Math.round(X/3600)}h ago`}function CX(X){return X.replace(/_/g," ").replace(/\b\w/g,(Z)=>Z.toUpperCase())}function BX(X){if(!X)return[];return Object.keys(X).filter((Z)=>X[Z]).map(CX)}function jX(X){if(!X)return"—";return new Date(X).toLocaleString(void 0,{hour12:!1})}function $Z(X){let Z=(X||"").toLowerCase();if(Z==="error"||Z==="critical"||Z==="crit"||Z==="fault")return"err";if(Z==="warn"||Z==="warning")return"warn";return"info"}class BZ extends G{static properties={power:{type:Number},cap:{type:Number}};constructor(){super();this.power=0,this.cap=0}static styles=J`
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
  `;pct(){if(!(this.cap>0))return 0;return Math.max(0,Math.min(100,this.power/this.cap*100))}render(){let X=this.pct(),Z=$X(X),K=90,Y=Math.PI*90,Q=Y*(1-X/100);return $`
      <div class="wrap">
        <svg viewBox="0 0 200 120" role="img" aria-label="fleet output gauge">
          <path
            class="track"
            d="M10 110 A 90 90 0 0 1 190 110"
            fill="none"
            stroke-width="14"
          />
          <path
            class="arc ${Z}"
            d="M10 110 A 90 90 0 0 1 190 110"
            fill="none"
            stroke-width="14"
            stroke-dasharray="${Y}"
            stroke-dashoffset="${Q}"
          />
        </svg>
        <div class="center">
          <div class="big">${V(this.power)}</div>
          <div class="sub">${g(X)} of ${V(this.cap)}</div>
        </div>
      </div>
    `}}customElements.define("fleet-gauge",BZ);class jZ extends G{static properties={label:{type:String},value:{type:String},sub:{type:String}};constructor(){super();this.label="",this.value="",this.sub=""}static styles=J`
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
  `;render(){return $`
      <div class="label">${this.label}</div>
      <div class="value">${this.value}</div>
      ${this.sub?$`<div class="sub">${this.sub}</div>`:""}
    `}}customElements.define("stat-card",jZ);class zZ extends G{static properties={inverter:{attribute:!1},name:{type:String},profile:{type:String}};constructor(){super();this.name="",this.profile=""}static styles=J`
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
    .model {
      font-weight: 600;
      font-size: 15px;
    }
    .uid {
      color: var(--muted);
      font-size: 12px;
      font-family: var(--mono);
    }
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
    .dot {
      width: 9px;
      height: 9px;
      border-radius: 50%;
      display: inline-block;
      margin-right: 6px;
    }
    .dot.on {
      background: var(--ok);
      box-shadow: 0 0 6px var(--ok);
    }
    .dot.off {
      background: var(--muted);
    }
    .state {
      font-size: 12px;
      color: var(--muted);
    }
    .power {
      display: flex;
      align-items: baseline;
      gap: 8px;
    }
    .pw {
      font-size: 28px;
      font-weight: 700;
      color: var(--text);
    }
    .cap {
      color: var(--muted);
      font-size: 13px;
    }
    .bar {
      height: 8px;
      background: var(--bar-bg);
      border-radius: 4px;
      overflow: hidden;
      margin: 10px 0 14px;
    }
    .fill {
      height: 100%;
      border-radius: 4px;
      transition: width 0.4s ease;
    }
    .fill.low { background: var(--ok); }
    .fill.mid { background: var(--accent); }
    .fill.high { background: var(--warn); }
    .fill.idle { background: var(--muted); }
    .metrics {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 8px;
      font-size: 13px;
    }
    .metric .k { color: var(--muted); font-size: 11px; }
    .metric .v { color: var(--text); font-weight: 600; }
    .panels {
      margin-top: 14px;
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(76px, 1fr));
      gap: 6px;
    }
    .panel {
      background: var(--bar-bg);
      border-radius: 6px;
      padding: 6px 8px;
      font-size: 11px;
    }
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
  `;render(){let X=this.inverter;if(!X)return z;let Z=$X(X.load_pct),K=BX(X.faults),Y=Math.max(0,Math.min(100,X.load_pct));return $`
      <div class="head">
        <div>
          <div class="model">${this.name||X.model||"unknown"}</div>
          <div class="uid">${this.name?`${X.model} · ${X.uid}`:X.uid}</div>
          ${this.profile?$`<div class="profile" title="Local Site profile active">⚙ ${this.profile}</div>`:z}
        </div>
        <div class="state">
          <span class="dot ${X.online?"on":"off"}"></span>
          ${X.online?"online":"offline"} · ${QZ(X.age_s)}
        </div>
      </div>

      <div class="power">
        <span class="pw">${V(X.active_power_w)}</span>
        <span class="cap">/ ${X.nameplate_w} W · ${g(X.load_pct)}</span>
      </div>
      <div class="bar"><div class="fill ${Z}" style="width:${Y}%"></div></div>

      <div class="metrics">
        <div class="metric"><div class="k">Grid</div><div class="v">${i(X.grid_v)}</div></div>
        <div class="metric"><div class="k">Freq</div><div class="v">${QX(X.freq_hz)}</div></div>
        <div class="metric"><div class="k">RSSI / LQI</div><div class="v">${X.rssi} / ${X.lqi}</div></div>
      </div>

      ${X.panels?.length?$`<div class="panels">
            ${X.panels.map((Q)=>$`<div class="panel">
                <div class="pi">DC ${Q.index+1}</div>
                <div class="pw">${V(Q.w)}</div>
                <div>${i(Q.dc_v)} · ${YZ(Q.dc_a)}</div>
              </div>`)}
          </div>`:z}

      ${K.length?$`<div class="chips">
            ${K.map((Q)=>$`<span class="chip">${Q}</span>`)}
          </div>`:z}
    `}}customElements.define("inverter-card",zZ);class GZ extends G{static properties={system:{attribute:!1}};constructor(){super();this.system=null}static styles=J`
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
  `;idRow(X,Z){return Z?$`<div class="k">${X}</div><div class="v">${Z}</div>`:z}clients(){let X=new Map;for(let Z of this.system?.peers??[]){let K=X.get(Z.backend)??{backend:Z.backend,version:Z.version,controller:!1,conns:0};if(K.conns++,K.controller=K.controller||Z.controller,Z.version)K.version=Z.version;X.set(Z.backend,K)}return[...X.values()].sort((Z,K)=>Z.backend.localeCompare(K.backend))}render(){let X=this.system,Z=X?.ecu,K=this.clients(),Y=!!(Z&&(Z.ecu_id||Z.hostname));return $`
      ${Y?$`<div class="id">
            ${this.idRow("ECU ID",Z.ecu_id)}
            ${this.idRow("Host",Z.hostname)}
          </div>`:z}

      <div class="peers">
        ${K.length?K.map((Q)=>$`<div class="peer">
                <span class="dot on"></span>
                <span class="name">${Q.backend||"(unnamed)"}</span>
                ${Q.controller?$`<span class="role ctl">ctrl</span>`:z}
                ${Q.conns>1?$`<span class="role">${Q.conns} conns</span>`:z}
                <span class="ver">${Q.version||""}</span>
              </div>`):$`<div class="empty">No peers connected.</div>`}
      </div>

      ${X?.status_error?$`<div class="warn">⚠ ${X.status_error}</div>`:z}
    `}}customElements.define("ecu-clients-card",GZ);function Q5(X,Z,K){if(X.length<2)return{line:"",area:"",max:0};let Y=X[0].t,Q=Math.max(1,X[X.length-1].t-Y),B=Math.max(1,...X.map((k)=>k.w)),j=(k)=>[(k.t-Y)/Q*Z,K-k.w/B*K],H="";for(let k=0;k<X.length;k++){let[N,W]=j(X[k]);H+=`${k===0?"M":"L"}${N.toFixed(1)} ${W.toFixed(1)} `}let[U]=j(X[0]),[_]=j(X[X.length-1]),q=`${H}L${_.toFixed(1)} ${K} L${U.toFixed(1)} ${K} Z`;return{line:H.trim(),area:q,max:B}}var zX=600,r=160;class HZ extends G{static properties={points:{attribute:!1},hoverIdx:{state:!0}};constructor(){super();this.points=[],this.hoverIdx=-1}static styles=J`
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
  `;onMove=(X)=>{let Z=this.points.length;if(Z<2)return;let Y=X.currentTarget.clientWidth||1,Q=Math.min(1,Math.max(0,X.offsetX/Y));this.hoverIdx=Math.round(Q*(Z-1))};onLeave=()=>{this.hoverIdx=-1};render(){let X=this.points??[];if(X.length<2)return $`<div class="empty">Collecting power history…</div>`;let{line:Z,area:K,max:Y}=Q5(X,zX,r),Q=X[X.length-1].w,B=this.hoverIdx,j=B>=0&&B<X.length,H=X[0].t,U=Math.max(1,X[X.length-1].t-H),_=j?(X[B].t-H)/U*zX:0,q=j?r-X[B].w/Y*r:0;return $`
      <div class="wrap">
        <svg
          viewBox="0 0 ${zX} ${r}"
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
          ${KX`<path class="area" d=${K} />`}
          ${KX`<path class="line" d=${Z} />`}
          ${j?KX`<line class="cross" x1=${_} y1="0" x2=${_} y2=${r} /><circle class="cursor" cx=${_} cy=${q} r="3.5" />`:z}
        </svg>
        ${j?$`<div class="tip" style="left:${_/zX*100}%; top:${q}px">
              <span class="w">${V(X[B].w)}</span>
              <span class="t">· ${jX(X[B].t)}</span>
            </div>`:z}
      </div>
      <div class="labels">
        <span>now <span class="cur">${V(Q)}</span></span>
        <span>peak ${V(Y)}</span>
      </div>
    `}}customElements.define("power-chart",HZ);class JZ extends G{static properties={fleet:{attribute:!1},system:{attribute:!1},names:{attribute:!1},profiles:{attribute:!1},history:{state:!0}};timer=null;constructor(){super();this.fleet=null,this.system=null,this.names={},this.profiles={},this.history=[]}connectedCallback(){super.connectedCallback(),this.loadHistory(),this.timer=setInterval(()=>void this.loadHistory(),60000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async loadHistory(){try{this.history=await M.history()}catch{}}chartPoints(){if(!this.fleet)return this.history;return[...this.history,{t:Date.now(),w:this.fleet.active_power_w}]}static styles=J`
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
    .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
    @media (max-width: 720px) { .grid, .stats { grid-template-columns: 1fr; } }
  `;render(){let X=this.fleet;if(!X)return $`<div class="empty">Waiting for inv-driver…</div>`;return $`
      <div class="grid">
        <div class="panel">
          <h2>Array output</h2>
          <fleet-gauge .power=${X.active_power_w} .cap=${X.nameplate_total_w}></fleet-gauge>
          <div class="online">${X.online_count} / ${X.inverter_count} inverters online</div>
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
        <stat-card label="Today" value=${s(X.today_wh)}></stat-card>
        <stat-card label="This month" value=${s(X.month_wh)}></stat-card>
        <stat-card label="This year" value=${s(X.year_wh)}></stat-card>
        <stat-card label="Lifetime" value=${s(X.lifetime_wh)}></stat-card>
      </div>

      <h2>Inverters</h2>
      ${X.inverters.length?$`<div class="cards">
            ${X.inverters.map((Z)=>$`<inverter-card
                .inverter=${Z}
                .name=${this.names?.[Z.uid]??""}
                .profile=${this.profiles?.[Z.uid]??""}
              ></inverter-card>`)}
          </div>`:$`<div class="empty">No inverters discovered yet.</div>`}
      ${z}
    `}}customElements.define("dashboard-view",JZ);class qZ extends G{static properties={fleet:{attribute:!1},names:{attribute:!1}};constructor(){super();this.fleet=null,this.names={}}rename(X,Z){let K=Z.target.value;this.dispatchEvent(new CustomEvent("rename",{detail:{uid:X,name:K},bubbles:!0,composed:!0}))}static styles=J`
    :host { display: block; }
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
    .fw { font-variant-numeric: tabular-nums; color: var(--muted); }
    .fault { color: var(--err); }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
  `;render(){let X=this.fleet;if(!X||X.inverters.length===0)return $`<div class="empty">No inverters discovered yet.</div>`;return $`
      <table>
        <thead>
          <tr>
            <th>Inverter ID</th><th>Name</th><th>Model</th><th>Firmware</th><th>Status</th>
            <th class="num">Output</th><th class="num">Load</th>
            <th class="num">Grid</th><th class="num">Freq</th>
            <th class="num">Panels</th><th class="num">Faults</th>
          </tr>
        </thead>
        <tbody>
          ${X.inverters.map((Z)=>{let K=Z.faults?Object.values(Z.faults).filter(Boolean).length:0;return $`<tr>
              <td class="uid">${Z.uid}</td>
              <td>
                <input
                  class="name-in"
                  .value=${this.names?.[Z.uid]??""}
                  placeholder="add a name"
                  @change=${(Y)=>this.rename(Z.uid,Y)}
                />
              </td>
              <td>${Z.model||"—"}</td>
              <td class="fw">${Z.sw_version||"—"}</td>
              <td>
                <span class="dot ${Z.online?"on":"off"}"></span>${Z.online?"online":"offline"}
              </td>
              <td class="num">${V(Z.active_power_w)} / ${Z.nameplate_w} W</td>
              <td class="num">${g(Z.load_pct)}</td>
              <td class="num">${i(Z.grid_v)}</td>
              <td class="num">${QX(Z.freq_hz)}</td>
              <td class="num">${Z.panels?.length??0}</td>
              <td class="num ${K?"fault":""}">${K||"—"}</td>
            </tr>`})}
        </tbody>
      </table>
    `}}customElements.define("inverters-view",qZ);class FZ extends G{static properties={fleet:{attribute:!1}};constructor(){super();this.fleet=null}static styles=J`
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
  `;alarms(){let X=[];for(let Z of this.fleet?.inverters??[]){for(let K of BX(Z.faults))X.push({uid:Z.uid,model:Z.model,label:K,severity:"fault"});if(!Z.online)X.push({uid:Z.uid,model:Z.model,label:"Inverter offline",severity:"warning"})}return X}render(){let X=this.alarms();if(X.length===0)return $`<div class="ok"><div class="big">✓ No active alarms</div><div>All inverters reporting healthy.</div></div>`;return $`${X.map((Z)=>$`<div class="row ${Z.severity}">
        <span class="sev">${Z.severity}</span>
        <span class="label">${Z.label} <span style="color:var(--muted)">· ${Z.model||"?"}</span></span>
        <span class="uid">${Z.uid}</span>
      </div>`)}`}}customElements.define("alarms-view",FZ);class UZ extends G{static properties={events:{attribute:!1}};constructor(){super();this.events=[]}static styles=J`
    :host { display: block; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { text-align: left; padding: 9px 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
    th { color: var(--muted); text-transform: uppercase; font-size: 11px; letter-spacing: 0.04em; }
    td { color: var(--text); }
    .time { color: var(--muted); white-space: nowrap; font-variant-numeric: tabular-nums; }
    .uid { font-family: var(--mono); color: var(--muted); font-size: 12px; }
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
  `;render(){if(!this.events||this.events.length===0)return $`<div class="empty">No events recorded.</div>`;return $`
      <table>
        <thead>
          <tr><th>Time</th><th>Severity</th><th>Event</th><th>Inverter</th><th>Detail</th></tr>
        </thead>
        <tbody>
          ${this.events.map((X)=>$`<tr>
              <td class="time">${jX(X.ts_ms)}</td>
              <td><span class="sev ${$Z(X.severity)}">${X.severity}</span></td>
              <td>${CX(X.kind)}</td>
              <td class="uid">${X.inverter_uid||"—"}</td>
              <td class="detail">${X.detail||(X.raw_hex?X.raw_hex:z)}</td>
            </tr>`)}
        </tbody>
      </table>
    `}}customElements.define("events-table",UZ);class kZ extends G{static properties={events:{state:!0},error:{state:!0},loading:{state:!0}};timer=null;constructor(){super();this.events=[],this.error="",this.loading=!1}static styles=J`
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
  `;connectedCallback(){super.connectedCallback(),this.load(),this.timer=setInterval(()=>void this.load(),15000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async load(){this.loading=!0;try{let X=await M.events({limit:200});this.events=X.events??[],this.error=X.error??""}catch(X){this.error=X.message}finally{this.loading=!1}}render(){return $`
      <div class="bar">
        <span class="count">${this.events.length} event(s)${this.loading?" · refreshing…":""}</span>
        <button @click=${()=>void this.load()}>Refresh</button>
      </div>
      ${this.error?$`<div class="err">⚠ ${this.error}</div>`:z}
      <div class="panel"><events-table .events=${this.events}></events-table></div>
    `}}customElements.define("events-view",kZ);class MZ extends G{static properties={profiles:{attribute:!1},activeBase:{attribute:!1},reconcilerReady:{attribute:!1},busy:{attribute:!1},selected:{state:!0}};constructor(){super();this.profiles=[],this.activeBase="",this.reconcilerReady=!0,this.busy=!1,this.selected=""}static styles=J`
    :host { display: block; }
    .grid { display: grid; gap: 16px; max-width: 460px; }
    .active { font-size: 14px; color: var(--text); }
    .active .muted { color: var(--muted); }
    .active .none { color: var(--muted); font-style: italic; }
    label { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: var(--muted); }
    select {
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
  `;onChange=(X)=>{this.selected=X.target.value};apply=()=>{let X=this.effectiveSelected();if(!X||X===this.activeBase)return;this.dispatchEvent(new CustomEvent("apply",{detail:X,bubbles:!0,composed:!0}))};effectiveSelected(){return this.selected||this.activeBase}labelFor(X){let Z=[`${X.vnom_v} V`];if(X.source_ref)Z.push(X.source_ref);return Z.push(`${X.point_count} pts`),`${X.id} — ${Z.join(" · ")}`}render(){let X=this.effectiveSelected(),Z=this.profiles.find((Y)=>Y.id===this.activeBase),K=!this.busy&&this.reconcilerReady&&X!==""&&X!==this.activeBase;return $`
      <div class="grid">
        <div class="active">
          <span class="muted">Active profile:</span>
          ${this.activeBase?$` <strong>${this.activeBase}</strong>${Z?$` <span class="muted">(${Z.vnom_v} V · ${Z.point_count} pts)</span>`:z}`:$` <span class="none">none selected</span>`}
        </div>

        <label>
          Base profile
          <select id="profile" .value=${X} @change=${this.onChange} ?disabled=${this.busy}>
            ${this.activeBase?z:$`<option value="" disabled selected>Select a profile…</option>`}
            ${this.profiles.map((Y)=>$`<option value=${Y.id} ?selected=${Y.id===X}>${this.labelFor(Y)}</option>`)}
          </select>
        </label>

        <div class="actions">
          <button class="apply" @click=${this.apply} ?disabled=${!K}>
            ${this.busy?"Applying…":"Apply"}
          </button>
          ${!this.reconcilerReady?$`<span class="hint">reconciler not ready</span>`:X&&X!==this.activeBase?$`<span class="hint">applies to all inverters</span>`:z}
        </div>
      </div>
    `}}customElements.define("grid-profile-form",MZ);var WZ={AC:{label:"Undervoltage trip — stage 2",desc:"Disconnect when AC voltage drops to this lower-stage level."},AQ:{label:"Undervoltage trip — deep",desc:"Disconnect quickly when voltage falls this far below nominal."},AH:{label:"Undervoltage trip — fast",desc:"Fast disconnect on a severe undervoltage."},AD:{label:"Overvoltage trip — slow",desc:"Disconnect when AC voltage rises above this (slower stage)."},AY:{label:"Overvoltage trip — slow (stage 2)",desc:"Second slower overvoltage disconnect threshold."},AB:{label:"10-minute mean overvoltage",desc:"Trips if the 10-minute average voltage exceeds this (EN 50549 sustained-overvoltage limit)."},AI:{label:"Overvoltage trip — fast",desc:"Fast disconnect on a severe overvoltage."},AE:{label:"Underfrequency trip — slow",desc:"Disconnect when grid frequency falls below this (slower stage)."},AJ:{label:"Underfrequency trip — fast",desc:"Fast disconnect on a severe underfrequency."},AF:{label:"Overfrequency trip — slow",desc:"Disconnect when grid frequency rises above this (slower stage)."},AK:{label:"Overfrequency trip — fast",desc:"Fast disconnect on a severe overfrequency."},BB:{label:"Undervoltage 1 — clearance time",desc:"How long the undervoltage condition must persist before tripping."},BD:{label:"Undervoltage 2 — clearance time",desc:"Clearance delay for the second undervoltage stage."},BC:{label:"Overvoltage 1 — clearance time",desc:"How long the overvoltage condition must persist before tripping."},BE:{label:"Overvoltage 2 — clearance time",desc:"Clearance delay for the second overvoltage stage."},BH:{label:"Underfrequency 1 — clearance time",desc:"Clearance delay for the first underfrequency stage."},BJ:{label:"Underfrequency 2 — clearance time",desc:"Clearance delay for the second underfrequency stage."},BI:{label:"Overfrequency 1 — clearance time",desc:"Clearance delay for the first overfrequency stage."},BK:{label:"Overfrequency 2 — clearance time",desc:"Clearance delay for the second overfrequency stage."},BN:{label:"Enter-service voltage — lower",desc:"Voltage must be above this before the inverter reconnects."},BO:{label:"Enter-service voltage — upper",desc:"Voltage must be below this before the inverter reconnects."},BP:{label:"Enter-service frequency — lower",desc:"Frequency must be above this before the inverter reconnects."},BQ:{label:"Enter-service frequency — upper",desc:"Frequency must be below this before the inverter reconnects."},AG:{label:"Grid-recovery delay",desc:"Wait time after the grid is healthy before reconnecting."},AS:{label:"Power ramp time",desc:"Time taken to ramp output back up after reconnecting."},CV:{label:"Over-freq curtailment — enable",desc:"Enables over-frequency power reduction (0 = off, 1 = on)."},CA:{label:"Over-freq curtailment — start",desc:"Frequency at which power reduction begins (deadband end)."},DD:{label:"Over-freq curtailment — slope",desc:"Power reduced per Hz above the start frequency (droop gradient)."},CG:{label:"Over-freq curtailment — response time",desc:"Filter/response time of the curtailment control loop."},DH:{label:"Under-freq Watt — low point",desc:"Lower frequency point of the under-frequency power curve."},DI:{label:"Under-freq Watt — high point",desc:"Upper frequency point of the under-frequency power curve."},CB:{label:"Over-freq Watt — start point",desc:"Frequency where over-frequency power reduction begins."},CC:{label:"Over-freq Watt — end point",desc:"Frequency where over-frequency power reduction reaches its limit."}},DX={DERFreqDroop:{label:"Frequency-Watt droop",tip:"Linearly reduces active power as frequency rises above a deadband — over-frequency curtailment (SunSpec DERFreqDroop, model 711)."},CrvSet:{label:"Frequency-Watt curve",tip:"Legacy point-based power-versus-frequency response curve (model 134)."},MustTrip:{label:"Trip thresholds",tip:"Voltage and frequency limits that disconnect the inverter from the grid when crossed (protection trips)."},DEREnterService:{label:"Enter service",tip:"The voltage/frequency window and timing the inverter must satisfy before (re)connecting after a trip."}},NX=["DERFreqDroop","CrvSet","MustTrip","DEREnterService"],_Z=new Set(["MustTrip","DEREnterService"]);function $5(X,Z){if(!X)return Z;return X.replace(/_/g," ").replace(/\b\w/g,(K)=>K.toUpperCase())}function AZ(X,Z){return WZ[X]?.label??$5(Z??"",X)}function IZ(X){return WZ[X]?.desc??""}var B5=[{left:"DH",right:"DI",message:"Under-frequency Watt: the low point (DH) must be below the high point (DI)."},{left:"CB",right:"CC",message:"Over-frequency Watt: the start point (CB) must be below the end point (CC)."},{left:"BN",right:"BO",message:"Enter-service voltage: the lower limit (BN) must be below the upper limit (BO)."},{left:"BP",right:"BQ",message:"Enter-service frequency: the lower limit (BP) must be below the upper limit (BQ)."},{left:"CA",right:"AF",message:"Over-frequency curtailment start (CA) must be below the over-frequency trip (AF), or the inverter trips instead of curtailing."}];function RX(X){let Z=[];for(let K of B5){let Y=X(K.left),Q=X(K.right);if(Y!==void 0&&Q!==void 0&&!(Y<Q))Z.push(K.message)}return Z}class OZ extends G{static properties={params:{attribute:!1},inverters:{attribute:!1},defaults:{attribute:!1},profile:{attribute:!1},names:{attribute:!1},busy:{attribute:!1},editing:{attribute:!1},name:{state:!0},selectedUids:{state:!0},values:{state:!0},localError:{state:!0}};constructor(){super();this.params=[],this.inverters=[],this.defaults={},this.profile=null,this.names={},this.busy=!1,this.editing=!1,this.name="",this.selectedUids=[],this.values={},this.localError=""}static styles=J`
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

    .legend { display: flex; flex-wrap: wrap; gap: 8px; }
    .badge {
      font-size: 11px; font-weight: 600; border-radius: 999px; padding: 2px 9px;
      background: var(--bar-bg); border: 1px solid var(--border); color: var(--muted); cursor: help;
    }

    details.group { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
    details.group + details.group { margin-top: 10px; }
    summary { list-style: none; cursor: pointer; padding: 10px 14px; display: flex; align-items: center; gap: 10px; background: var(--bar-bg); }
    summary::-webkit-details-marker { display: none; }
    summary .gname { font-size: 14px; font-weight: 600; color: var(--text); }
    summary .gcount { font-size: 12px; color: var(--muted); margin-left: auto; }
    summary .badge { cursor: help; }

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
  `;willUpdate(X){if(X.has("profile")){let Z=this.profile;this.name=Z?.id??"",this.selectedUids=[...Z?.uids??[]];let K={};for(let Y of Z?.points??[])K[Y.aps_code]=String(Y.value);this.values=K,this.localError=""}}effectiveWritable(){if(!this.selectedUids.length)return new Set;let X=this.selectedUids.map((K)=>new Set(this.inverters.find((Y)=>Y.uid===K)?.writable_codes??[])),Z=X[0];for(let K of X.slice(1))Z=new Set([...Z].filter((Y)=>K.has(Y)));return Z}effectiveValue(X){let Z=(this.values[X]??"").trim();if(Z!==""&&!Number.isNaN(Number(Z)))return Number(Z);return this.defaults[X]?.value}outOfRange(X){let Z=(this.values[X]??"").trim();if(Z===""||Number.isNaN(Number(Z)))return!1;let K=this.defaults[X];if(!K)return!1;let Y=Number(Z);return K.min!==void 0&&Y<K.min||K.max!==void 0&&Y>K.max}label(X){return this.names[X.uid]||X.model||X.uid}toggleTarget(X,Z){this.selectedUids=Z?[...this.selectedUids,X]:this.selectedUids.filter((K)=>K!==X)}setValue(X,Z){this.values={...this.values,[X]:Z}}groups(){let X={};for(let K of this.params)(X[K.group]??=[]).push(K);return[...NX,...Object.keys(X).filter((K)=>!NX.includes(K))].filter((K)=>X[K]?.length).map((K)=>[K,X[K]])}save=()=>{let X=this.effectiveWritable(),Z=this.params.filter((Y)=>X.has(Y.aps_code)).map((Y)=>({p:Y,raw:(this.values[Y.aps_code]??"").trim()})).filter((Y)=>Y.raw!==""&&!Number.isNaN(Number(Y.raw))).map((Y)=>({aps_code:Y.p.aps_code,value:Number(Y.raw)}));if(!this.name.trim())return void(this.localError="Profile name is required.");if(!this.selectedUids.length)return void(this.localError="Select at least one target inverter.");if(!Z.length)return void(this.localError="Set at least one parameter value.");if(RX((Y)=>this.effectiveValue(Y)).length)return void(this.localError="Resolve the conflicts before saving.");this.localError="";let K={id:this.name.trim(),uids:this.selectedUids,points:Z};this.dispatchEvent(new CustomEvent("save",{detail:K,bubbles:!0,composed:!0}))};cancel=()=>this.dispatchEvent(new CustomEvent("cancel",{bubbles:!0,composed:!0}));renderRow(X,Z){let K=Z.has(X.aps_code),Y=this.defaults[X.aps_code],Q=(this.values[X.aps_code]??"").trim(),B=K&&Q!==""&&(!Y||Number(Q)!==Y.value),j=K&&this.outOfRange(X.aps_code);return $`<tr class="${K?"":"off"} ${B?"over":""}">
      <td>
        <div class="plabel">${AZ(X.aps_code,X.long_name)}${B?$`<span class="otag">overridden</span>`:z}</div>
        <div class="pdesc">${IZ(X.aps_code)} <span class="pcode">${X.aps_code}</span></div>
      </td>
      <td class="def">${Y?`${Y.value} ${Y.unit}`:"—"}</td>
      <td class="val">
        <input
          type="number" step="any" ?disabled=${!K}
          .value=${this.values[X.aps_code]??""}
          placeholder=${Y?String(Y.value):K?"—":"n/a"}
          @input=${(H)=>this.setValue(X.aps_code,H.target.value)}
        />
        <span class="unit">${X.unit}</span>
        ${j?$`<span class="warn">⚠ outside base range${Y?.min!==void 0?` (${Y.min}–${Y.max} ${Y.unit})`:""}</span>`:z}
      </td>
    </tr>`}render(){let X=this.effectiveWritable(),Z=this.selectedUids.length>0,K=Z?RX((Y)=>this.effectiveValue(Y)):[];return $`
      <div class="grid">
        <label class="field">
          Profile name
          <input type="text" .value=${this.name} ?disabled=${this.editing} placeholder="e.g. victron-shift"
            @input=${(Y)=>this.name=Y.target.value} />
        </label>

        <fieldset>
          <legend>Target inverters</legend>
          <div class="targets">
            ${this.inverters.length===0?$`<span class="hint">No inverters seen yet.</span>`:this.inverters.map((Y)=>$`<label class="target">
                    <input type="checkbox" .checked=${this.selectedUids.includes(Y.uid)}
                      @change=${(Q)=>this.toggleTarget(Y.uid,Q.target.checked)} />
                    ${this.label(Y)} <span class="pcode">${Y.model}</span>
                  </label>`)}
          </div>
        </fieldset>

        ${!Z?$`<span class="hint">Select a target to choose editable parameters.</span>`:$`
              ${K.length?$`<div class="conflicts">⚠ Conflicting settings — resolve to save:
                    <ul>${K.map((Y)=>$`<li>${Y}</li>`)}</ul>
                  </div>`:z}

              <div class="legend">
                ${this.groups().map(([Y])=>{let Q=DX[Y];return $`<span class="badge" title=${Q?.tip??Y}>${Q?.label??Y}</span>`})}
              </div>

              ${this.groups().map(([Y,Q])=>{let B=DX[Y];return $`<details class="group" ?open=${!_Z.has(Y)}>
                  <summary>
                    <span class="gname">${B?.label??Y}</span>
                    <span class="badge" title=${B?.tip??Y}>${Y}</span>
                    <span class="gcount">${Q.length} setting${Q.length===1?"":"s"}</span>
                  </summary>
                  <table>
                    <thead><tr><th>Setting</th><th>Base default</th><th>Override</th></tr></thead>
                    <tbody>${Q.map((j)=>this.renderRow(j,X))}</tbody>
                  </table>
                </details>`})}

              ${this.selectedUids.length>1?$`<div class="hint">Greyed rows are not writable on every selected target.</div>`:z}
            `}

        ${this.localError?$`<div class="err">⚠ ${this.localError}</div>`:z}

        <div class="actions">
          <button class="save" @click=${this.save} ?disabled=${this.busy||K.length>0}>
            ${this.busy?"Applying…":"Save & apply"}
          </button>
          <button class="cancel" @click=${this.cancel} ?disabled=${this.busy}>Cancel</button>
          <span class="hint">${K.length?"resolve conflicts to save":"applies to the selected inverters"}</span>
        </div>
      </div>
    `}}customElements.define("local-site-profile-form",OZ);class TZ extends G{static properties={data:{state:!0},names:{state:!0},error:{state:!0},notice:{state:!0},baseBusy:{state:!0},overlayBusy:{state:!0},editing:{state:!0},editingExisting:{state:!0}};constructor(){super();this.data=null,this.names={},this.error="",this.notice="",this.baseBusy=!1,this.overlayBusy=!1,this.editing=null,this.editingExisting=!1}static styles=J`
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
    button.primary { background: var(--accent); border: none; color: #04121a; border-radius: 8px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; }
    button.primary:hover { filter: brightness(1.08); }
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
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){try{let[X,Z]=await Promise.all([M.profiles(),M.getSettings()]);this.data=X,this.error=X.error??"",this.names=Z.settings?.inverter_names??{}}catch(X){this.error=X.message}}invName(X){if(this.names[X])return this.names[X];return this.data?.inverters.find((K)=>K.uid===X)?.model||X}onSelectBase=async(X)=>{let Z=X.detail;if(!window.confirm(`Apply base grid profile "${Z}" to every inverter? This writes grid-protection settings across the whole fleet.`))return;this.baseBusy=!0,this.notice="",this.error="";try{await M.selectBase(Z),await this.load(),this.notice=`Base profile "${Z}" applied.`}catch(K){this.error=K.message}finally{this.baseBusy=!1}};newProfile(){this.editing={id:"",uids:[],points:[]},this.editingExisting=!1,this.notice="",this.error=""}editProfile(X){this.editing=X,this.editingExisting=!0,this.notice="",this.error=""}onCancelEdit=()=>{this.editing=null};onSaveOverlay=async(X)=>{let Z=X.detail;if(!window.confirm(`Apply Local Site profile "${Z.id}" to ${Z.uids.length} inverter(s)? This writes grid-protection parameters to each.`))return;this.overlayBusy=!0,this.notice="",this.error="";try{let K=await M.saveOverlay(Z);this.editing=null,await this.load(),this.reportResults(Z.id,K.results)}catch(K){this.error=K.message}finally{this.overlayBusy=!1}};deleteProfile=async(X)=>{if(!window.confirm(`Delete Local Site profile "${X.id}" and clear it from ${X.uids.length} inverter(s)?`))return;this.overlayBusy=!0,this.notice="",this.error="";try{let Z=await M.deleteOverlay(X.id,X.uids);if(this.editing?.id===X.id)this.editing=null;await this.load(),this.reportResults(X.id,Z.results,"cleared")}catch(Z){this.error=Z.message}finally{this.overlayBusy=!1}};reportResults(X,Z,K="applied"){let Y=Z.filter((Q)=>!Q.ok);if(Y.length===0)this.notice=`Profile "${X}" ${K} to ${Z.length} inverter(s).`;else{let Q=K==="cleared"?"clearing":"applying",B=Y.map((j)=>`${this.invName(j.uid)}: ${j.error||"unconfirmed"}`).join("; ");this.notice=`Profile "${X}" saved on the ECU, but ${Q} was not confirmed on ${Y.length} of ${Z.length} inverter(s) (offline?) — ${B}`}}renderBase(){let X=this.data?.base;return $`
      <div class="panel">
        <h2>Base grid profile</h2>
        <grid-profile-form
          .profiles=${X?.profiles??[]}
          .activeBase=${X?.active_base??""}
          .reconcilerReady=${X?.reconciler_ready??!1}
          .busy=${this.baseBusy}
          @apply=${this.onSelectBase}
        ></grid-profile-form>
      </div>
    `}renderLocalSite(){let X=this.data;return $`
      <div class="panel">
        <div class="row">
          <h2 style="margin:0">Local Site profiles</h2>
          ${this.editing===null?$`<button class="primary" @click=${()=>this.newProfile()}>+ New profile</button>`:z}
        </div>

        ${this.editing!==null?$`<local-site-profile-form
              .params=${X?.params??[]}
              .inverters=${X?.inverters??[]}
              .defaults=${X?.base_defaults??{}}
              .names=${this.names}
              .profile=${this.editing}
              .editing=${this.editingExisting}
              .busy=${this.overlayBusy}
              @save=${this.onSaveOverlay}
              @cancel=${this.onCancelEdit}
            ></local-site-profile-form>`:this.renderCards()}
      </div>
    `}renderCards(){let X=this.data?.overlays??[];if(X.length===0)return $`<div class="empty">No Local Site profiles yet. Create one to override grid-protection parameters on specific inverters.</div>`;return $`<div class="cards">
      ${X.map((Z)=>$`<div class="card">
          <div class="title">${Z.id}</div>
          <div class="meta">Targets: ${Z.uids.map((K)=>this.invName(K)).join(", ")||"none"}</div>
          <div class="chips">
            ${Z.points.map((K)=>$`<span class="chip">${K.aps_code} = ${K.value}${K.unit?` ${K.unit}`:""}</span>`)}
          </div>
          <div class="cardactions">
            <button @click=${()=>this.editProfile(Z)}>Edit</button>
            <button class="del" @click=${()=>this.deleteProfile(Z)}>Delete</button>
          </div>
        </div>`)}
    </div>`}render(){return $`
      ${this.notice?$`<div class="banner ok">${this.notice}</div>`:z}
      ${this.error?$`<div class="banner err">⚠ ${this.error}</div>`:z}
      ${this.data===null?$`<div class="panel"><div class="loading">Loading…</div></div>`:$`<div class="cols">
            <div>${this.renderLocalSite()}</div>
            <div>${this.renderBase()}</div>
          </div>`}
    `}}customElements.define("profiles-view",TZ);class VZ extends G{static properties={settings:{attribute:!1}};constructor(){super();this.settings={ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}static styles=J`
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
  `;save=()=>{let X=this.shadowRoot;if(!X)return;let Z=(Y)=>(X.querySelector(`#${Y}`)?.value??"").trim(),K={ecu_id:Z("ecu_id"),mac:Z("mac"),pan_override:Z("pan_override"),zigbee_type:Z("zigbee_type")};this.dispatchEvent(new CustomEvent("save",{detail:K,bubbles:!0,composed:!0}))};render(){let X=this.settings;return $`
      <div class="grid">
        <label>
          ECU ID
          <input id="ecu_id" type="text" .value=${X.ecu_id??""} />
        </label>
        <label>
          MAC
          <input id="mac" type="text" .value=${X.mac??""} />
        </label>
        <label>
          PAN override
          <input id="pan_override" type="text" placeholder="auto from MAC" .value=${X.pan_override??""} />
        </label>
        <label>
          ZigBee type
          <select id="zigbee_type" .value=${X.zigbee_type||"apsystems"}>
            <option value="apsystems">apsystems</option>
            <option value="general">general</option>
          </select>
        </label>
        <div class="actions">
          <button class="save" @click=${this.save}>Save</button>
        </div>
      </div>
    `}}customElements.define("settings-form",VZ);class CZ extends G{static properties={settings:{state:!0},error:{state:!0},notice:{state:!0},loading:{state:!0},saving:{state:!0}};constructor(){super();this.settings=null,this.error="",this.notice="",this.loading=!1,this.saving=!1}static styles=J`
    :host { display: block; }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      max-width: 560px;
    }
    h2 { font-size: 15px; margin: 0 0 16px; color: var(--text); }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){this.loading=!0;try{let X=await M.getSettings();this.settings=X.settings??null,this.error=X.error??""}catch(X){this.error=X.message}finally{this.loading=!1}}onSave=async(X)=>{this.saving=!0,this.notice="",this.error="";try{this.settings=await M.saveSettings(X.detail),this.notice="Settings saved."}catch(Z){this.error=Z.message}finally{this.saving=!1,await this.load()}};render(){return $`
      <div class="panel">
        <h2>ECU settings</h2>
        ${this.notice?$`<div class="banner ok">${this.notice}</div>`:z}
        ${this.error?$`<div class="banner err">⚠ ${this.error}</div>`:z}
        ${this.loading&&!this.settings?$`<div class="loading">Loading…</div>`:$`<settings-form
              .settings=${this.settings??{ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}
              @save=${this.onSave}
            ></settings-form>`}
      </div>
    `}}customElements.define("settings-view",CZ);class DZ extends G{static properties={items:{attribute:!1},route:{type:String},open:{type:Boolean}};constructor(){super();this.items=[],this.route="dashboard",this.open=!1}close=()=>{this.dispatchEvent(new CustomEvent("close",{bubbles:!0,composed:!0}))};static styles=J`
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
  `;render(){return $`
      <nav class=${this.open?"open":""}>
        <div class="brand">ECU CONSOLE</div>
        ${this.items.map((X)=>$`<a
            class="item ${this.route===X.id?"active":""}"
            href="#/${X.id}"
            @click=${this.close}
          ><span class="ic">${X.icon}</span>${X.label}</a>`)}
      </nav>
      ${this.open?$`<div class="scrim" @click=${this.close}></div>`:z}
    `}}customElements.define("app-nav",DZ);var LX=[{id:"dashboard",label:"Dashboard",icon:"▮▮"},{id:"inverters",label:"Inverters",icon:"⌁"},{id:"alarms",label:"Alarms",icon:"!"},{id:"events",label:"Events",icon:"≣"},{id:"profiles",label:"Profiles",icon:"⛭"},{id:"settings",label:"Settings",icon:"⚙"}];class NZ extends G{static properties={ready:{state:!0},authed:{state:!0},configured:{state:!0},route:{state:!0},fleet:{state:!0},system:{state:!0},names:{state:!0},customProfiles:{state:!0},navOpen:{state:!0}};closeSSE=null;sysTimer=null;settingsCache=null;constructor(){super();this.ready=!1,this.authed=!1,this.configured=!0,this.route="dashboard",this.fleet=null,this.system=null,this.names={},this.customProfiles={},this.navOpen=!1}static styles=J`
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
      .layout { grid-template-columns: 1fr; }
      button.hamburger { display: inline-flex; }
      main { padding: 18px 16px; }
    }
  `;connectedCallback(){super.connectedCallback(),window.addEventListener("hashchange",this.onHash),this.onHash(),this.init()}disconnectedCallback(){super.disconnectedCallback(),window.removeEventListener("hashchange",this.onHash),this.stopStreams()}onHash=()=>{let X=(location.hash.replace(/^#\/?/,"")||"dashboard").split("/")[0];if(this.route=LX.some((Z)=>Z.id===X)?X:"dashboard",this.navOpen=!1,this.route==="dashboard"&&this.authed)this.fetchOverlays()};async init(){try{let X=await M.authStatus();if(this.configured=X.configured,this.authed=X.authenticated,this.authed)this.startStreams()}catch{}finally{this.ready=!0}}onAuthed=async()=>{this.authed=!0,this.startStreams()};logout=async()=>{try{await M.logout()}catch{}this.authed=!1,this.stopStreams(),this.fleet=null,this.system=null};startStreams(){this.stopStreams(),this.closeSSE=ZZ((Z)=>{this.fleet=Z});let X=()=>M.system().then((Z)=>this.system=Z).catch(()=>{});X(),this.sysTimer=setInterval(X,5000),this.fetchSettings(),this.fetchOverlays()}async fetchSettings(){try{let X=await M.getSettings();if(X.settings)this.settingsCache=X.settings,this.names=X.settings.inverter_names??{}}catch{}}async fetchOverlays(){try{let X=await M.overlays(),Z={};for(let K of X)for(let Y of K.uids)Z[Y]=K.id;this.customProfiles=Z}catch{}}onRename=async(X)=>{let{uid:Z,name:K}=X.detail,Y=this.settingsCache??{ecu_id:"",mac:"",pan_override:"",zigbee_type:""},Q={...Y.inverter_names??{}};if(K.trim())Q[Z]=K.trim();else delete Q[Z];let B={...Y,inverter_names:Q};try{await M.saveSettings(B),this.settingsCache=B,this.names=Q}catch{}};stopStreams(){if(this.closeSSE?.(),this.closeSSE=null,this.sysTimer)clearInterval(this.sysTimer);this.sysTimer=null}activeView(){switch(this.route){case"inverters":return $`<inverters-view
          .fleet=${this.fleet}
          .names=${this.names}
          @rename=${this.onRename}
        ></inverters-view>`;case"alarms":return $`<alarms-view .fleet=${this.fleet}></alarms-view>`;case"events":return $`<events-view></events-view>`;case"profiles":return $`<profiles-view></profiles-view>`;case"settings":return $`<settings-view></settings-view>`;default:return $`<dashboard-view
          .fleet=${this.fleet}
          .system=${this.system}
          .names=${this.names}
          .profiles=${this.customProfiles}
        ></dashboard-view>`}}render(){if(!this.ready)return z;if(!this.authed)return $`<login-view .configured=${this.configured} @authed=${this.onAuthed}></login-view>`;let X=LX.find((K)=>K.id===this.route)?.label??"Dashboard",Z=this.system?.invdriver_connected??!1;return $`
      <div class="layout">
        <app-nav
          .items=${LX}
          .route=${this.route}
          .open=${this.navOpen}
          @close=${()=>this.navOpen=!1}
        ></app-nav>
        <main>
          <div class="topbar">
            <div class="titlewrap">
              <button class="hamburger" aria-label="Menu" aria-expanded=${this.navOpen} @click=${()=>this.navOpen=!this.navOpen}>☰</button>
              <h1>${X}</h1>
            </div>
            <div class="right">
              <span class="conn">
                <span class="dot ${Z?"on":"off"}"></span>
                inv-driver ${Z?"connected":"down"}
              </span>
              <button class="logout" @click=${this.logout}>Sign out</button>
            </div>
          </div>
          ${this.activeView()}
        </main>
      </div>
    `}}customElements.define("ecu-app",NZ);
