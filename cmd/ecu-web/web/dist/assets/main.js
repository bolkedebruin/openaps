var a=globalThis,jX=a.ShadowRoot&&(a.ShadyCSS===void 0||a.ShadyCSS.nativeShadow)&&"adoptedStyleSheets"in Document.prototype&&"replace"in CSSStyleSheet.prototype,zX=Symbol(),RX=new WeakMap;class HX{constructor(X,Y,Z){if(this._$cssResult$=!0,Z!==zX)throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");this.cssText=X,this._strings=Y}get styleSheet(){let X=this._styleSheet,Y=this._strings;if(jX&&X===void 0){let Z=Y!==void 0&&Y.length===1;if(Z)X=RX.get(Y);if(X===void 0){if((this._styleSheet=X=new CSSStyleSheet).replaceSync(this.cssText),Z)RX.set(Y,X)}}return X}toString(){return this.cssText}}var kY=(X)=>{if(X._$cssResult$===!0)return X.cssText;else if(typeof X==="number")return X;else throw Error(`Value passed to 'css' function must be a 'css' function result: ${X}. Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.`)},MY=(X)=>new HX(typeof X==="string"?X:String(X),void 0,zX),U=(X,...Y)=>{let Z=X.length===1?X[0]:Y.reduce((K,Q,B)=>K+kY(Q)+X[B+1],X[0]);return new HX(Z,X,zX)},VX=(X,Y)=>{if(jX)X.adoptedStyleSheets=Y.map((Z)=>Z instanceof CSSStyleSheet?Z:Z.styleSheet);else for(let Z of Y){let K=document.createElement("style"),Q=a.litNonce;if(Q!==void 0)K.setAttribute("nonce",Q);K.textContent=Z.cssText,X.appendChild(K)}},WY=(X)=>{let Y="";for(let Z of X.cssRules)Y+=Z.cssText;return MY(Y)},JX=jX?(X)=>X:(X)=>X instanceof CSSStyleSheet?WY(X):X;var{is:_Y,defineProperty:OY,getOwnPropertyDescriptor:LX,getOwnPropertyNames:AY,getOwnPropertySymbols:IY,getPrototypeOf:SX}=Object,TY=!1,O=globalThis;if(TY)O.customElements??=customElements;var A=!0,N,PX=O.trustedTypes,CY=PX?PX.emptyScript:"",yX=A?O.reactiveElementPolyfillSupportDevMode:O.reactiveElementPolyfillSupport;if(A)O.litIssuedWarnings??=new Set,N=(X,Y)=>{if(Y+=` See https://lit.dev/msg/${X} for more information.`,!O.litIssuedWarnings.has(Y)&&!O.litIssuedWarnings.has(X))console.warn(Y),O.litIssuedWarnings.add(Y)},queueMicrotask(()=>{if(N("dev-mode","Lit is in dev mode. Not recommended for production!"),O.ShadyDOM?.inUse&&yX===void 0)N("polyfill-support-missing","Shadow DOM is being polyfilled via `ShadyDOM` but the `polyfill-support` module has not been loaded.")});var NY=A?(X)=>{if(!O.emitLitDebugLogEvents)return;O.dispatchEvent(new CustomEvent("lit-debug",{detail:X}))}:void 0,E=(X,Y)=>X,FX={toAttribute(X,Y){switch(Y){case Boolean:X=X?CY:null;break;case Object:case Array:X=X==null?X:JSON.stringify(X);break}return X},fromAttribute(X,Y){let Z=X;switch(Y){case Boolean:Z=X!==null;break;case Number:Z=X===null?null:Number(X);break;case Object:case Array:try{Z=JSON.parse(X)}catch(K){Z=null}break}return Z}},xX=(X,Y)=>!_Y(X,Y),bX={attribute:!0,type:String,converter:FX,reflect:!1,useDefault:!1,hasChanged:xX};Symbol.metadata??=Symbol("metadata");O.litPropertyMetadata??=new WeakMap;class I extends HTMLElement{static addInitializer(X){this.__prepare(),(this._initializers??=[]).push(X)}static get observedAttributes(){return this.finalize(),this.__attributeToPropertyMap&&[...this.__attributeToPropertyMap.keys()]}static createProperty(X,Y=bX){if(Y.state)Y.attribute=!1;if(this.__prepare(),this.prototype.hasOwnProperty(X))Y=Object.create(Y),Y.wrapped=!0;if(this.elementProperties.set(X,Y),!Y.noAccessor){let Z=A?Symbol.for(`${String(X)} (@property() cache)`):Symbol(),K=this.getPropertyDescriptor(X,Z,Y);if(K!==void 0)OY(this.prototype,X,K)}}static getPropertyDescriptor(X,Y,Z){let{get:K,set:Q}=LX(this.prototype,X)??{get(){return this[Y]},set(B){this[Y]=B}};if(A&&K==null){if("value"in(LX(this.prototype,X)??{}))throw Error(`Field ${JSON.stringify(String(X))} on ${this.name} was declared as a reactive property but it's actually declared as a value on the prototype. Usually this is due to using @property or @state on a method.`);N("reactive-property-without-getter",`Field ${JSON.stringify(String(X))} on ${this.name} was declared as a reactive property but it does not have a getter. This will be an error in a future version of Lit.`)}return{get:K,set(B){let $=K?.call(this);Q?.call(this,B),this.requestUpdate(X,$,Z)},configurable:!0,enumerable:!0}}static getPropertyOptions(X){return this.elementProperties.get(X)??bX}static __prepare(){if(this.hasOwnProperty(E("elementProperties",this)))return;let X=SX(this);if(X.finalize(),X._initializers!==void 0)this._initializers=[...X._initializers];this.elementProperties=new Map(X.elementProperties)}static finalize(){if(this.hasOwnProperty(E("finalized",this)))return;if(this.finalized=!0,this.__prepare(),this.hasOwnProperty(E("properties",this))){let Y=this.properties,Z=[...AY(Y),...IY(Y)];for(let K of Z)this.createProperty(K,Y[K])}let X=this[Symbol.metadata];if(X!==null){let Y=litPropertyMetadata.get(X);if(Y!==void 0)for(let[Z,K]of Y)this.elementProperties.set(Z,K)}this.__attributeToPropertyMap=new Map;for(let[Y,Z]of this.elementProperties){let K=this.__attributeNameForProperty(Y,Z);if(K!==void 0)this.__attributeToPropertyMap.set(K,Y)}if(this.elementStyles=this.finalizeStyles(this.styles),A){if(this.hasOwnProperty("createProperty"))N("no-override-create-property","Overriding ReactiveElement.createProperty() is deprecated. The override will not be called with standard decorators");if(this.hasOwnProperty("getPropertyDescriptor"))N("no-override-get-property-descriptor","Overriding ReactiveElement.getPropertyDescriptor() is deprecated. The override will not be called with standard decorators")}}static finalizeStyles(X){let Y=[];if(Array.isArray(X)){let Z=new Set(X.flat(1/0).reverse());for(let K of Z)Y.unshift(JX(K))}else if(X!==void 0)Y.push(JX(X));return Y}static __attributeNameForProperty(X,Y){let Z=Y.attribute;return Z===!1?void 0:typeof Z==="string"?Z:typeof X==="string"?X.toLowerCase():void 0}constructor(){super();this.__instanceProperties=void 0,this.isUpdatePending=!1,this.hasUpdated=!1,this.__reflectingProperty=null,this.__initialize()}__initialize(){this.__updatePromise=new Promise((X)=>this.enableUpdating=X),this._$changedProperties=new Map,this.__saveInstanceProperties(),this.requestUpdate(),this.constructor._initializers?.forEach((X)=>X(this))}addController(X){if((this.__controllers??=new Set).add(X),this.renderRoot!==void 0&&this.isConnected)X.hostConnected?.()}removeController(X){this.__controllers?.delete(X)}__saveInstanceProperties(){let X=new Map,Y=this.constructor.elementProperties;for(let Z of Y.keys())if(this.hasOwnProperty(Z))X.set(Z,this[Z]),delete this[Z];if(X.size>0)this.__instanceProperties=X}createRenderRoot(){let X=this.shadowRoot??this.attachShadow(this.constructor.shadowRootOptions);return VX(X,this.constructor.elementStyles),X}connectedCallback(){this.renderRoot??=this.createRenderRoot(),this.enableUpdating(!0),this.__controllers?.forEach((X)=>X.hostConnected?.())}enableUpdating(X){}disconnectedCallback(){this.__controllers?.forEach((X)=>X.hostDisconnected?.())}attributeChangedCallback(X,Y,Z){this._$attributeToProperty(X,Z)}__propertyToAttribute(X,Y){let K=this.constructor.elementProperties.get(X),Q=this.constructor.__attributeNameForProperty(X,K);if(Q!==void 0&&K.reflect===!0){let $=(K.converter?.toAttribute!==void 0?K.converter:FX).toAttribute(Y,K.type);if(A&&this.constructor.enabledWarnings.includes("migration")&&$===void 0)N("undefined-attribute-value",`The attribute value for the ${X} property is undefined on element ${this.localName}. The attribute will be removed, but in the previous version of \`ReactiveElement\`, the attribute would not have changed.`);if(this.__reflectingProperty=X,$==null)this.removeAttribute(Q);else this.setAttribute(Q,$);this.__reflectingProperty=null}}_$attributeToProperty(X,Y){let Z=this.constructor,K=Z.__attributeToPropertyMap.get(X);if(K!==void 0&&this.__reflectingProperty!==K){let Q=Z.getPropertyOptions(K),B=typeof Q.converter==="function"?{fromAttribute:Q.converter}:Q.converter?.fromAttribute!==void 0?Q.converter:FX;this.__reflectingProperty=K;let $=B.fromAttribute(Y,Q.type);this[K]=$??this.__defaultValues?.get(K)??$,this.__reflectingProperty=null}}requestUpdate(X,Y,Z,K=!1,Q){if(X!==void 0){if(A&&X instanceof Event)N("","The requestUpdate() method was called with an Event as the property name. This is probably a mistake caused by binding this.requestUpdate as an event listener. Instead bind a function that will call it with no arguments: () => this.requestUpdate()");let B=this.constructor;if(K===!1)Q=this[X];if(Z??=B.getPropertyOptions(X),(Z.hasChanged??xX)(Q,Y)||Z.useDefault&&Z.reflect&&Q===this.__defaultValues?.get(X)&&!this.hasAttribute(B.__attributeNameForProperty(X,Z)))this._$changeProperty(X,Y,Z);else return}if(this.isUpdatePending===!1)this.__updatePromise=this.__enqueueUpdate()}_$changeProperty(X,Y,{useDefault:Z,reflect:K,wrapped:Q},B){if(Z&&!(this.__defaultValues??=new Map).has(X)){if(this.__defaultValues.set(X,B??Y??this[X]),Q!==!0||B!==void 0)return}if(!this._$changedProperties.has(X)){if(!this.hasUpdated&&!Z)Y=void 0;this._$changedProperties.set(X,Y)}if(K===!0&&this.__reflectingProperty!==X)(this.__reflectingProperties??=new Set).add(X)}async __enqueueUpdate(){this.isUpdatePending=!0;try{await this.__updatePromise}catch(Y){Promise.reject(Y)}let X=this.scheduleUpdate();if(X!=null)await X;return!this.isUpdatePending}scheduleUpdate(){let X=this.performUpdate();if(A&&this.constructor.enabledWarnings.includes("async-perform-update")&&typeof X?.then==="function")N("async-perform-update",`Element ${this.localName} returned a Promise from performUpdate(). This behavior is deprecated and will be removed in a future version of ReactiveElement.`);return X}performUpdate(){if(!this.isUpdatePending)return;if(NY?.({kind:"update"}),!this.hasUpdated){if(this.renderRoot??=this.createRenderRoot(),A){let Q=[...this.constructor.elementProperties.keys()].filter((B)=>this.hasOwnProperty(B)&&(B in SX(this)));if(Q.length)throw Error(`The following properties on element ${this.localName} will not trigger updates as expected because they are set using class fields: ${Q.join(", ")}. Native class fields and some compiled output will overwrite accessors used for detecting changes. See https://lit.dev/msg/class-field-shadowing for more information.`)}if(this.__instanceProperties){for(let[K,Q]of this.__instanceProperties)this[K]=Q;this.__instanceProperties=void 0}let Z=this.constructor.elementProperties;if(Z.size>0)for(let[K,Q]of Z){let{wrapped:B}=Q,$=this[K];if(B===!0&&!this._$changedProperties.has(K)&&$!==void 0)this._$changeProperty(K,void 0,Q,$)}}let X=!1,Y=this._$changedProperties;try{if(X=this.shouldUpdate(Y),X)this.willUpdate(Y),this.__controllers?.forEach((Z)=>Z.hostUpdate?.()),this.update(Y);else this.__markUpdated()}catch(Z){throw X=!1,this.__markUpdated(),Z}if(X)this._$didUpdate(Y)}willUpdate(X){}_$didUpdate(X){if(this.__controllers?.forEach((Y)=>Y.hostUpdated?.()),!this.hasUpdated)this.hasUpdated=!0,this.firstUpdated(X);if(this.updated(X),A&&this.isUpdatePending&&this.constructor.enabledWarnings.includes("change-in-update"))N("change-in-update",`Element ${this.localName} scheduled an update (generally because a property was set) after an update completed, causing a new update to be scheduled. This is inefficient and should be avoided unless the next update can only be scheduled as a side effect of the previous update.`)}__markUpdated(){this._$changedProperties=new Map,this.isUpdatePending=!1}get updateComplete(){return this.getUpdateComplete()}getUpdateComplete(){return this.__updatePromise}shouldUpdate(X){return!0}update(X){this.__reflectingProperties&&=this.__reflectingProperties.forEach((Y)=>this.__propertyToAttribute(Y,this[Y])),this.__markUpdated()}updated(X){}firstUpdated(X){}}I.elementStyles=[];I.shadowRootOptions={mode:"open"};I[E("elementProperties",I)]=new Map;I[E("finalized",I)]=new Map;yX?.({ReactiveElement:I});if(A){I.enabledWarnings=["change-in-update","async-perform-update"];let X=function(Y){if(!Y.hasOwnProperty(E("enabledWarnings",Y)))Y.enabledWarnings=Y.enabledWarnings.slice()};I.enableWarning=function(Y){if(X(this),!this.enabledWarnings.includes(Y))this.enabledWarnings.push(Y)},I.disableWarning=function(Y){X(this);let Z=this.enabledWarnings.indexOf(Y);if(Z>=0)this.enabledWarnings.splice(Z,1)}}(O.reactiveElementVersions??=[]).push("2.1.2");if(A&&O.reactiveElementVersions.length>1)queueMicrotask(()=>{N("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var T=globalThis,F=(X)=>{if(!T.emitLitDebugLogEvents)return;T.dispatchEvent(new CustomEvent("lit-debug",{detail:X}))},DY=0,m;T.litIssuedWarnings??=new Set,m=(X,Y)=>{if(Y+=X?` See https://lit.dev/msg/${X} for more information.`:"",!T.litIssuedWarnings.has(Y)&&!T.litIssuedWarnings.has(X))console.warn(Y),T.litIssuedWarnings.add(Y)},queueMicrotask(()=>{m("dev-mode","Lit is in dev mode. Not recommended for production!")});var D=T.ShadyDOM?.inUse&&T.ShadyDOM?.noPatch===!0?T.ShadyDOM.wrap:(X)=>X,n=T.trustedTypes,fX=n?n.createPolicy("lit-html",{createHTML:(X)=>X}):void 0,RY=(X)=>X,YX=(X,Y,Z)=>RY,VY=(X)=>{if(f!==YX)throw Error("Attempted to overwrite existing lit-html security policy. setSanitizeDOMValueFactory should be called at most once.");f=X},LY=()=>{f=YX},WX=(X,Y,Z)=>{return f(X,Y,Z)},mX="$lit$",V=`lit$${Math.random().toFixed(9).slice(2)}$`,uX="?"+V,SY=`<${uX}>`,y=document,u=()=>y.createComment(""),v=(X)=>X===null||typeof X!="object"&&typeof X!="function",_X=Array.isArray,PY=(X)=>_X(X)||typeof X?.[Symbol.iterator]==="function",UX=`[ 	
\f\r]`,bY=`[^ 	
\f\r"'\`<>=]`,yY=`[^\\s"'>=/]`,g=/<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g,EX=1,qX=2,xY=3,wX=/-->/g,hX=/>/g,S=new RegExp(`>|${UX}(?:(${yY}+)(${UX}*=${UX}*(?:${bY}|("|')|))|$)`,"g"),fY=0,cX=1,EY=2,gX=3,kX=/'/g,MX=/"/g,vX=/^(?:script|style|textarea|title)$/i,wY=1,t=2,e=3,OX=1,XX=2,hY=3,cY=4,gY=5,AX=6,dY=7,IX=(X)=>(Y,...Z)=>{if(Y.some((K)=>K===void 0))console.warn(`Some template strings are undefined.
This is probably caused by illegal octal escape sequences.`);if(Z.some((K)=>K?._$litStatic$))m("",`Static values 'literal' or 'unsafeStatic' cannot be used as values to non-static templates.
Please use the static 'html' tag function. See https://lit.dev/docs/templates/expressions/#static-expressions`);return{["_$litType$"]:X,strings:Y,values:Z}},G=IX(wY),ZX=IX(t),nY=IX(e),x=Symbol.for("lit-noChange"),j=Symbol.for("lit-nothing"),dX=new WeakMap,b=y.createTreeWalker(y,129),f=YX;function pX(X,Y){if(!_X(X)||!X.hasOwnProperty("raw")){let Z="invalid template strings array";throw Z=`
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
`),Error(Z)}return fX!==void 0?fX.createHTML(Y):Y}var mY=(X,Y)=>{let Z=X.length-1,K=[],Q=Y===t?"<svg>":Y===e?"<math>":"",B,$=g;for(let q=0;q<Z;q++){let W=X[q],J=-1,k,R=0,M;while(R<W.length){if($.lastIndex=R,M=$.exec(W),M===null)break;if(R=$.lastIndex,$===g){if(M[EX]==="!--")$=wX;else if(M[EX]!==void 0)$=hX;else if(M[qX]!==void 0){if(vX.test(M[qX]))B=new RegExp(`</${M[qX]}`,"g");$=S}else if(M[xY]!==void 0)throw Error("Bindings in tag names are not supported. Please use static templates instead. See https://lit.dev/docs/templates/expressions/#static-expressions")}else if($===S)if(M[fY]===">")$=B??g,J=-1;else if(M[cX]===void 0)J=-2;else J=$.lastIndex-M[EY].length,k=M[cX],$=M[gX]===void 0?S:M[gX]==='"'?MX:kX;else if($===MX||$===kX)$=S;else if($===wX||$===hX)$=g;else $=S,B=void 0}console.assert(J===-1||$===S||$===kX||$===MX,"unexpected parse state B");let P=$===S&&X[q+1].startsWith("/>")?" ":"";Q+=$===g?W+SY:J>=0?(K.push(k),W.slice(0,J)+mX+W.slice(J))+V+P:W+V+(J===-2?q:P)}let z=Q+(X[Z]||"<?>")+(Y===t?"</svg>":Y===e?"</math>":"");return[pX(X,z),K]};class p{constructor({strings:X,["_$litType$"]:Y},Z){this.parts=[];let K,Q=0,B=0,$=X.length-1,z=this.parts,[q,W]=mY(X,Y);if(this.el=p.createElement(q,Z),b.currentNode=this.el.content,Y===t||Y===e){let J=this.el.content.firstChild;J.replaceWith(...J.childNodes)}while((K=b.nextNode())!==null&&z.length<$){if(K.nodeType===1){{let J=K.localName;if(/^(?:textarea|template)$/i.test(J)&&K.innerHTML.includes(V)){let k=`Expressions are not supported inside \`${J}\` elements. See https://lit.dev/msg/expression-in-${J} for more information.`;if(J==="template")throw Error(k);else m("",k)}}if(K.hasAttributes()){for(let J of K.getAttributeNames())if(J.endsWith(mX)){let k=W[B++],M=K.getAttribute(J).split(V),P=/([.?@])?(.*)/.exec(k);z.push({type:OX,index:Q,name:P[2],strings:M,ctor:P[1]==="."?lX:P[1]==="?"?iX:P[1]==="@"?sX:l}),K.removeAttribute(J)}else if(J.startsWith(V))z.push({type:AX,index:Q}),K.removeAttribute(J)}if(vX.test(K.tagName)){let J=K.textContent.split(V),k=J.length-1;if(k>0){K.textContent=n?n.emptyScript:"";for(let R=0;R<k;R++)K.append(J[R],u()),b.nextNode(),z.push({type:XX,index:++Q});K.append(J[k],u())}}}else if(K.nodeType===8)if(K.data===uX)z.push({type:XX,index:Q});else{let k=-1;while((k=K.data.indexOf(V,k+1))!==-1)z.push({type:dY,index:Q}),k+=V.length-1}Q++}if(W.length!==B)throw Error('Detected duplicate attribute bindings. This occurs if your template has duplicate attributes on an element tag. For example "<input ?disabled=${true} ?disabled=${false}>" contains a duplicate "disabled" attribute. The error was detected in the following template: \n`'+X.join("${...}")+"`");F&&F({kind:"template prep",template:this,clonableTemplate:this.el,parts:this.parts,strings:X})}static createElement(X,Y){let Z=y.createElement("template");return Z.innerHTML=X,Z}}function w(X,Y,Z=X,K){if(Y===x)return Y;let Q=K!==void 0?Z.__directives?.[K]:Z.__directive,B=v(Y)?void 0:Y._$litDirective$;if(Q?.constructor!==B){if(Q?._$notifyDirectiveConnectionChanged?.(!1),B===void 0)Q=void 0;else Q=new B(X),Q._$initialize(X,Z,K);if(K!==void 0)(Z.__directives??=[])[K]=Q;else Z.__directive=Q}if(Q!==void 0)Y=w(X,Q._$resolve(X,Y.values),Q,K);return Y}class oX{constructor(X,Y){this._$parts=[],this._$disconnectableChildren=void 0,this._$template=X,this._$parent=Y}get parentNode(){return this._$parent.parentNode}get _$isConnected(){return this._$parent._$isConnected}_clone(X){let{el:{content:Y},parts:Z}=this._$template,K=(X?.creationScope??y).importNode(Y,!0);b.currentNode=K;let Q=b.nextNode(),B=0,$=0,z=Z[0];while(z!==void 0){if(B===z.index){let q;if(z.type===XX)q=new o(Q,Q.nextSibling,this,X);else if(z.type===OX)q=new z.ctor(Q,z.name,z.strings,this,X);else if(z.type===AX)q=new rX(Q,this,X);this._$parts.push(q),z=Z[++$]}if(B!==z?.index)Q=b.nextNode(),B++}return b.currentNode=y,K}_update(X){let Y=0;for(let Z of this._$parts){if(Z!==void 0)if(F&&F({kind:"set part",part:Z,value:X[Y],valueIndex:Y,values:X,templateInstance:this}),Z.strings!==void 0)Z._$setValue(X,Z,Y),Y+=Z.strings.length-2;else Z._$setValue(X[Y]);Y++}}}class o{get _$isConnected(){return this._$parent?._$isConnected??this.__isConnected}constructor(X,Y,Z,K){this.type=XX,this._$committedValue=j,this._$disconnectableChildren=void 0,this._$startNode=X,this._$endNode=Y,this._$parent=Z,this.options=K,this.__isConnected=K?.isConnected??!0,this._textSanitizer=void 0}get parentNode(){let X=D(this._$startNode).parentNode,Y=this._$parent;if(Y!==void 0&&X?.nodeType===11)X=Y.parentNode;return X}get startNode(){return this._$startNode}get endNode(){return this._$endNode}_$setValue(X,Y=this){if(this.parentNode===null)throw Error("This `ChildPart` has no `parentNode` and therefore cannot accept a value. This likely means the element containing the part was manipulated in an unsupported way outside of Lit's control such that the part's marker nodes were ejected from DOM. For example, setting the element's `innerHTML` or `textContent` can do this.");if(X=w(this,X,Y),v(X)){if(X===j||X==null||X===""){if(this._$committedValue!==j)F&&F({kind:"commit nothing to child",start:this._$startNode,end:this._$endNode,parent:this._$parent,options:this.options}),this._$clear();this._$committedValue=j}else if(X!==this._$committedValue&&X!==x)this._commitText(X)}else if(X._$litType$!==void 0)this._commitTemplateResult(X);else if(X.nodeType!==void 0){if(this.options?.host===X){this._commitText("[probable mistake: rendered a template's host in itself (commonly caused by writing ${this} in a template]"),console.warn("Attempted to render the template host",X,"inside itself. This is almost always a mistake, and in dev mode ","we render some warning text. In production however, we'll ","render it, which will usually result in an error, and sometimes ","in the element disappearing from the DOM.");return}this._commitNode(X)}else if(PY(X))this._commitIterable(X);else this._commitText(X)}_insert(X){return D(D(this._$startNode).parentNode).insertBefore(X,this._$endNode)}_commitNode(X){if(this._$committedValue!==X){if(this._$clear(),f!==YX){let Y=this._$startNode.parentNode?.nodeName;if(Y==="STYLE"||Y==="SCRIPT"){let Z="Forbidden";if(Y==="STYLE")Z="Lit does not support binding inside style nodes. This is a security risk, as style injection attacks can exfiltrate data and spoof UIs. Consider instead using css`...` literals to compose styles, and do dynamic styling with css custom properties, ::parts, <slot>s, and by mutating the DOM rather than stylesheets.";else Z="Lit does not support binding inside script nodes. This is a security risk, as it could allow arbitrary code execution.";throw Error(Z)}}F&&F({kind:"commit node",start:this._$startNode,parent:this._$parent,value:X,options:this.options}),this._$committedValue=this._insert(X)}}_commitText(X){if(this._$committedValue!==j&&v(this._$committedValue)){let Y=D(this._$startNode).nextSibling;if(this._textSanitizer===void 0)this._textSanitizer=WX(Y,"data","property");X=this._textSanitizer(X),F&&F({kind:"commit text",node:Y,value:X,options:this.options}),Y.data=X}else{let Y=y.createTextNode("");if(this._commitNode(Y),this._textSanitizer===void 0)this._textSanitizer=WX(Y,"data","property");X=this._textSanitizer(X),F&&F({kind:"commit text",node:Y,value:X,options:this.options}),Y.data=X}this._$committedValue=X}_commitTemplateResult(X){let{values:Y,["_$litType$"]:Z}=X,K=typeof Z==="number"?this._$getTemplate(X):(Z.el===void 0&&(Z.el=p.createElement(pX(Z.h,Z.h[0]),this.options)),Z);if(this._$committedValue?._$template===K)F&&F({kind:"template updating",template:K,instance:this._$committedValue,parts:this._$committedValue._$parts,options:this.options,values:Y}),this._$committedValue._update(Y);else{let Q=new oX(K,this),B=Q._clone(this.options);F&&F({kind:"template instantiated",template:K,instance:Q,parts:Q._$parts,options:this.options,fragment:B,values:Y}),Q._update(Y),F&&F({kind:"template instantiated and updated",template:K,instance:Q,parts:Q._$parts,options:this.options,fragment:B,values:Y}),this._commitNode(B),this._$committedValue=Q}}_$getTemplate(X){let Y=dX.get(X.strings);if(Y===void 0)dX.set(X.strings,Y=new p(X));return Y}_commitIterable(X){if(!_X(this._$committedValue))this._$committedValue=[],this._$clear();let Y=this._$committedValue,Z=0,K;for(let Q of X){if(Z===Y.length)Y.push(K=new o(this._insert(u()),this._insert(u()),this,this.options));else K=Y[Z];K._$setValue(Q),Z++}if(Z<Y.length)this._$clear(K&&D(K._$endNode).nextSibling,Z),Y.length=Z}_$clear(X=D(this._$startNode).nextSibling,Y){this._$notifyConnectionChanged?.(!1,!0,Y);while(X!==this._$endNode){let Z=D(X).nextSibling;D(X).remove(),X=Z}}setConnected(X){if(this._$parent===void 0)this.__isConnected=X,this._$notifyConnectionChanged?.(X);else throw Error("part.setConnected() may only be called on a RootPart returned from render().")}}class l{get tagName(){return this.element.tagName}get _$isConnected(){return this._$parent._$isConnected}constructor(X,Y,Z,K,Q){if(this.type=OX,this._$committedValue=j,this._$disconnectableChildren=void 0,this.element=X,this.name=Y,this._$parent=K,this.options=Q,Z.length>2||Z[0]!==""||Z[1]!=="")this._$committedValue=Array(Z.length-1).fill(new String),this.strings=Z;else this._$committedValue=j;this._sanitizer=void 0}_$setValue(X,Y=this,Z,K){let Q=this.strings,B=!1;if(Q===void 0){if(X=w(this,X,Y,0),B=!v(X)||X!==this._$committedValue&&X!==x,B)this._$committedValue=X}else{let $=X;X=Q[0];let z,q;for(z=0;z<Q.length-1;z++){if(q=w(this,$[Z+z],Y,z),q===x)q=this._$committedValue[z];if(B||=!v(q)||q!==this._$committedValue[z],q===j)X=j;else if(X!==j)X+=(q??"")+Q[z+1];this._$committedValue[z]=q}}if(B&&!K)this._commitValue(X)}_commitValue(X){if(X===j)D(this.element).removeAttribute(this.name);else{if(this._sanitizer===void 0)this._sanitizer=f(this.element,this.name,"attribute");X=this._sanitizer(X??""),F&&F({kind:"commit attribute",element:this.element,name:this.name,value:X,options:this.options}),D(this.element).setAttribute(this.name,X??"")}}}class lX extends l{constructor(){super(...arguments);this.type=hY}_commitValue(X){if(this._sanitizer===void 0)this._sanitizer=f(this.element,this.name,"property");X=this._sanitizer(X),F&&F({kind:"commit property",element:this.element,name:this.name,value:X,options:this.options}),this.element[this.name]=X===j?void 0:X}}class iX extends l{constructor(){super(...arguments);this.type=cY}_commitValue(X){F&&F({kind:"commit boolean attribute",element:this.element,name:this.name,value:!!(X&&X!==j),options:this.options}),D(this.element).toggleAttribute(this.name,!!X&&X!==j)}}class sX extends l{constructor(X,Y,Z,K,Q){super(X,Y,Z,K,Q);if(this.type=gY,this.strings!==void 0)throw Error(`A \`<${X.localName}>\` has a \`@${Y}=...\` listener with invalid content. Event listeners in templates must have exactly one expression and no surrounding text.`)}_$setValue(X,Y=this){if(X=w(this,X,Y,0)??j,X===x)return;let Z=this._$committedValue,K=X===j&&Z!==j||X.capture!==Z.capture||X.once!==Z.once||X.passive!==Z.passive,Q=X!==j&&(Z===j||K);if(F&&F({kind:"commit event listener",element:this.element,name:this.name,value:X,options:this.options,removeListener:K,addListener:Q,oldListener:Z}),K)this.element.removeEventListener(this.name,this,Z);if(Q)this.element.addEventListener(this.name,this,X);this._$committedValue=X}handleEvent(X){if(typeof this._$committedValue==="function")this._$committedValue.call(this.options?.host??this.element,X);else this._$committedValue.handleEvent(X)}}class rX{constructor(X,Y,Z){this.element=X,this.type=AX,this._$disconnectableChildren=void 0,this._$parent=Y,this.options=Z}get _$isConnected(){return this._$parent._$isConnected}_$setValue(X){F&&F({kind:"commit to element binding",element:this.element,value:X,options:this.options}),w(this,X)}}var uY=T.litHtmlPolyfillSupportDevMode;uY?.(p,o);(T.litHtmlVersions??=[]).push("3.3.3");if(T.litHtmlVersions.length>1)queueMicrotask(()=>{m("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var d=(X,Y,Z)=>{if(Y==null)throw TypeError(`The container to render into may not be ${Y}`);let K=DY++,Q=Z?.renderBefore??Y,B=Q._$litPart$;if(F&&F({kind:"begin render",id:K,value:X,container:Y,options:Z,part:B}),B===void 0){let $=Z?.renderBefore??null;Q._$litPart$=B=new o(Y.insertBefore(u(),$),$,void 0,Z??{})}return B._$setValue(X),F&&F({kind:"end render",id:K,value:X,container:Y,options:Z,part:B}),B};d.setSanitizer=VY,d.createSanitizer=WX,d._testOnlyClearSanitizerFactoryDoNotCallOrElse=LY;var vY=(X,Y)=>X,TX=!0,L=globalThis,aX;if(TX)L.litIssuedWarnings??=new Set,aX=(X,Y)=>{if(Y+=` See https://lit.dev/msg/${X} for more information.`,!L.litIssuedWarnings.has(Y)&&!L.litIssuedWarnings.has(X))console.warn(Y),L.litIssuedWarnings.add(Y)};class H extends I{constructor(){super(...arguments);this.renderOptions={host:this},this.__childPart=void 0}createRenderRoot(){let X=super.createRenderRoot();return this.renderOptions.renderBefore??=X.firstChild,X}update(X){let Y=this.render();if(!this.hasUpdated)this.renderOptions.isConnected=this.isConnected;super.update(X),this.__childPart=d(Y,this.renderRoot,this.renderOptions)}connectedCallback(){super.connectedCallback(),this.__childPart?.setConnected(!0)}disconnectedCallback(){super.disconnectedCallback(),this.__childPart?.setConnected(!1)}render(){return x}}H._$litElement$=!0;H[vY("finalized",H)]=!0;L.litElementHydrateSupport?.({LitElement:H});var pY=TX?L.litElementPolyfillSupportDevMode:L.litElementPolyfillSupport;pY?.({LitElement:H});(L.litElementVersions??=[]).push("4.2.2");if(TX&&L.litElementVersions.length>1)queueMicrotask(()=>{aX("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});async function h(X){let Y=await fetch(X,{credentials:"same-origin"});if(!Y.ok)throw Error(`${X}: ${Y.status}`);return await Y.json()}async function CX(X,Y){let Z=await fetch(X,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Y)});if(!Z.ok){let K=await Z.text();throw Error(K.trim()||`${X}: ${Z.status}`)}}async function oY(X,Y){let Z=await fetch(X,{method:"PUT",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(Y)});if(!Z.ok){let K=await Z.text();throw Error(K.trim()||`${X}: ${Z.status}`)}return await Z.json()}var _={authStatus:()=>h("/api/auth/status"),setup:(X)=>CX("/api/auth/setup",{password:X}),login:(X)=>CX("/api/auth/login",{password:X}),logout:()=>CX("/api/auth/logout",{}),fleet:()=>h("/api/fleet"),system:()=>h("/api/system"),history:()=>h("/api/history"),events:(X={})=>{let Y=new URLSearchParams;if(X.since_ms)Y.set("since_ms",String(X.since_ms));if(X.kind)Y.set("kind",X.kind);if(X.severity)Y.set("severity",X.severity);if(X.inverter_uid)Y.set("inverter_uid",X.inverter_uid);if(X.limit)Y.set("limit",String(X.limit));let Z=Y.toString();return h("/api/events"+(Z?`?${Z}`:""))},getSettings:async()=>{let X=await h("/api/settings");if(X.error)return{error:X.error};return{settings:{ecu_id:X.ecu_id,mac:X.mac,pan_override:X.pan_override,zigbee_type:X.zigbee_type,inverter_names:X.inverter_names??{}}}},saveSettings:(X)=>oY("/api/settings",X)};function nX(X,Y){let Z=new EventSource("/api/stream");return Z.addEventListener("fleet",(K)=>{try{X(JSON.parse(K.data))}catch{}}),Z.onerror=()=>Y?.(),()=>Z.close()}class tX extends H{static properties={configured:{type:Boolean},error:{state:!0},busy:{state:!0}};constructor(){super();this.configured=!0,this.error="",this.busy=!1}static styles=U`
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
  `;async submit(X){X.preventDefault();let Z=this.renderRoot.querySelector("input")?.value??"";this.busy=!0,this.error="";try{if(this.configured)await _.login(Z);else await _.setup(Z);this.dispatchEvent(new CustomEvent("authed",{bubbles:!0,composed:!0}))}catch(K){this.error=K.message||"failed"}finally{this.busy=!1}}render(){return G`
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
    `}}customElements.define("login-view",tX);function C(X){if(!Number.isFinite(X))return"—";if(Math.abs(X)>=1000)return`${(X/1000).toFixed(2)} kW`;return`${Math.round(X)} W`}function i(X){if(!Number.isFinite(X))return"—";let Y=Math.abs(X);if(Y>=1e6)return`${(X/1e6).toFixed(2)} MWh`;if(Y>=1000)return`${(X/1000).toFixed(2)} kWh`;return`${Math.round(X)} Wh`}function c(X){return Number.isFinite(X)?`${X.toFixed(0)}%`:"—"}function s(X){return X>0?`${X.toFixed(1)} V`:"—"}function KX(X){return X>0?`${X.toFixed(2)} Hz`:"—"}function eX(X){return Number.isFinite(X)?`${X.toFixed(2)} A`:"—"}function QX(X){if(!(X>0))return"idle";if(X<40)return"low";if(X<85)return"mid";return"high"}function XY(X){if(!Number.isFinite(X)||X<0)return"—";if(X<60)return`${Math.round(X)}s ago`;if(X<3600)return`${Math.round(X/60)}m ago`;return`${Math.round(X/3600)}h ago`}function NX(X){return X.replace(/_/g," ").replace(/\b\w/g,(Y)=>Y.toUpperCase())}function GX(X){if(!X)return[];return Object.keys(X).filter((Y)=>X[Y]).map(NX)}function BX(X){if(!X)return"—";return new Date(X).toLocaleString(void 0,{hour12:!1})}function YY(X){let Y=(X||"").toLowerCase();if(Y==="error"||Y==="critical"||Y==="crit"||Y==="fault")return"err";if(Y==="warn"||Y==="warning")return"warn";return"info"}class ZY extends H{static properties={power:{type:Number},cap:{type:Number}};constructor(){super();this.power=0,this.cap=0}static styles=U`
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
  `;pct(){if(!(this.cap>0))return 0;return Math.max(0,Math.min(100,this.power/this.cap*100))}render(){let X=this.pct(),Y=QX(X),Z=90,K=Math.PI*90,Q=K*(1-X/100);return G`
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
            stroke-dasharray="${K}"
            stroke-dashoffset="${Q}"
          />
        </svg>
        <div class="center">
          <div class="big">${C(this.power)}</div>
          <div class="sub">${c(X)} of ${C(this.cap)}</div>
        </div>
      </div>
    `}}customElements.define("fleet-gauge",ZY);class KY extends H{static properties={label:{type:String},value:{type:String},sub:{type:String}};constructor(){super();this.label="",this.value="",this.sub=""}static styles=U`
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
  `;render(){return G`
      <div class="label">${this.label}</div>
      <div class="value">${this.value}</div>
      ${this.sub?G`<div class="sub">${this.sub}</div>`:""}
    `}}customElements.define("stat-card",KY);class QY extends H{static properties={inverter:{attribute:!1},name:{type:String}};constructor(){super();this.name=""}static styles=U`
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
  `;render(){let X=this.inverter;if(!X)return j;let Y=QX(X.load_pct),Z=GX(X.faults),K=Math.max(0,Math.min(100,X.load_pct));return G`
      <div class="head">
        <div>
          <div class="model">${this.name||X.model||"unknown"}</div>
          <div class="uid">${this.name?`${X.model} · ${X.uid}`:X.uid}</div>
        </div>
        <div class="state">
          <span class="dot ${X.online?"on":"off"}"></span>
          ${X.online?"online":"offline"} · ${XY(X.age_s)}
        </div>
      </div>

      <div class="power">
        <span class="pw">${C(X.active_power_w)}</span>
        <span class="cap">/ ${X.nameplate_w} W · ${c(X.load_pct)}</span>
      </div>
      <div class="bar"><div class="fill ${Y}" style="width:${K}%"></div></div>

      <div class="metrics">
        <div class="metric"><div class="k">Grid</div><div class="v">${s(X.grid_v)}</div></div>
        <div class="metric"><div class="k">Freq</div><div class="v">${KX(X.freq_hz)}</div></div>
        <div class="metric"><div class="k">RSSI / LQI</div><div class="v">${X.rssi} / ${X.lqi}</div></div>
      </div>

      ${X.panels?.length?G`<div class="panels">
            ${X.panels.map((Q)=>G`<div class="panel">
                <div class="pi">DC ${Q.index+1}</div>
                <div class="pw">${C(Q.w)}</div>
                <div>${s(Q.dc_v)} · ${eX(Q.dc_a)}</div>
              </div>`)}
          </div>`:j}

      ${Z.length?G`<div class="chips">
            ${Z.map((Q)=>G`<span class="chip">${Q}</span>`)}
          </div>`:j}
    `}}customElements.define("inverter-card",QY);class GY extends H{static properties={system:{attribute:!1}};constructor(){super();this.system=null}static styles=U`
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
  `;idRow(X,Y){return Y?G`<div class="k">${X}</div><div class="v">${Y}</div>`:j}clients(){let X=new Map;for(let Y of this.system?.peers??[]){let Z=X.get(Y.backend)??{backend:Y.backend,version:Y.version,controller:!1,conns:0};if(Z.conns++,Z.controller=Z.controller||Y.controller,Y.version)Z.version=Y.version;X.set(Y.backend,Z)}return[...X.values()].sort((Y,Z)=>Y.backend.localeCompare(Z.backend))}render(){let X=this.system,Y=X?.ecu,Z=this.clients(),K=!!(Y&&(Y.ecu_id||Y.hostname));return G`
      ${K?G`<div class="id">
            ${this.idRow("ECU ID",Y.ecu_id)}
            ${this.idRow("Host",Y.hostname)}
          </div>`:j}

      <div class="peers">
        ${Z.length?Z.map((Q)=>G`<div class="peer">
                <span class="dot on"></span>
                <span class="name">${Q.backend||"(unnamed)"}</span>
                ${Q.controller?G`<span class="role ctl">ctrl</span>`:j}
                ${Q.conns>1?G`<span class="role">${Q.conns} conns</span>`:j}
                <span class="ver">${Q.version||""}</span>
              </div>`):G`<div class="empty">No peers connected.</div>`}
      </div>

      ${X?.status_error?G`<div class="warn">⚠ ${X.status_error}</div>`:j}
    `}}customElements.define("ecu-clients-card",GY);function lY(X,Y,Z){if(X.length<2)return{line:"",area:"",max:0};let K=X[0].t,Q=Math.max(1,X[X.length-1].t-K),B=Math.max(1,...X.map((k)=>k.w)),$=(k)=>[(k.t-K)/Q*Y,Z-k.w/B*Z],z="";for(let k=0;k<X.length;k++){let[R,M]=$(X[k]);z+=`${k===0?"M":"L"}${R.toFixed(1)} ${M.toFixed(1)} `}let[q]=$(X[0]),[W]=$(X[X.length-1]),J=`${z}L${W.toFixed(1)} ${Z} L${q.toFixed(1)} ${Z} Z`;return{line:z.trim(),area:J,max:B}}var $X=600,r=160;class BY extends H{static properties={points:{attribute:!1},hoverIdx:{state:!0}};constructor(){super();this.points=[],this.hoverIdx=-1}static styles=U`
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
  `;onMove=(X)=>{let Y=this.points.length;if(Y<2)return;let K=X.currentTarget.clientWidth||1,Q=Math.min(1,Math.max(0,X.offsetX/K));this.hoverIdx=Math.round(Q*(Y-1))};onLeave=()=>{this.hoverIdx=-1};render(){let X=this.points??[];if(X.length<2)return G`<div class="empty">Collecting power history…</div>`;let{line:Y,area:Z,max:K}=lY(X,$X,r),Q=X[X.length-1].w,B=this.hoverIdx,$=B>=0&&B<X.length,z=X[0].t,q=Math.max(1,X[X.length-1].t-z),W=$?(X[B].t-z)/q*$X:0,J=$?r-X[B].w/K*r:0;return G`
      <div class="wrap">
        <svg
          viewBox="0 0 ${$X} ${r}"
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
          ${ZX`<path class="area" d=${Z} />`}
          ${ZX`<path class="line" d=${Y} />`}
          ${$?ZX`<line class="cross" x1=${W} y1="0" x2=${W} y2=${r} /><circle class="cursor" cx=${W} cy=${J} r="3.5" />`:j}
        </svg>
        ${$?G`<div class="tip" style="left:${W/$X*100}%; top:${J}px">
              <span class="w">${C(X[B].w)}</span>
              <span class="t">· ${BX(X[B].t)}</span>
            </div>`:j}
      </div>
      <div class="labels">
        <span>now <span class="cur">${C(Q)}</span></span>
        <span>peak ${C(K)}</span>
      </div>
    `}}customElements.define("power-chart",BY);class $Y extends H{static properties={fleet:{attribute:!1},system:{attribute:!1},names:{attribute:!1},history:{state:!0}};timer=null;constructor(){super();this.fleet=null,this.system=null,this.names={},this.history=[]}connectedCallback(){super.connectedCallback(),this.loadHistory(),this.timer=setInterval(()=>void this.loadHistory(),60000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async loadHistory(){try{this.history=await _.history()}catch{}}chartPoints(){if(!this.fleet)return this.history;return[...this.history,{t:Date.now(),w:this.fleet.active_power_w}]}static styles=U`
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
  `;render(){let X=this.fleet;if(!X)return G`<div class="empty">Waiting for inv-driver…</div>`;return G`
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
        <stat-card label="Today" value=${i(X.today_wh)}></stat-card>
        <stat-card label="This month" value=${i(X.month_wh)}></stat-card>
        <stat-card label="This year" value=${i(X.year_wh)}></stat-card>
        <stat-card label="Lifetime" value=${i(X.lifetime_wh)}></stat-card>
      </div>

      <h2>Inverters</h2>
      ${X.inverters.length?G`<div class="cards">
            ${X.inverters.map((Y)=>G`<inverter-card .inverter=${Y} .name=${this.names?.[Y.uid]??""}></inverter-card>`)}
          </div>`:G`<div class="empty">No inverters discovered yet.</div>`}
      ${j}
    `}}customElements.define("dashboard-view",$Y);class jY extends H{static properties={fleet:{attribute:!1},names:{attribute:!1}};constructor(){super();this.fleet=null,this.names={}}rename(X,Y){let Z=Y.target.value;this.dispatchEvent(new CustomEvent("rename",{detail:{uid:X,name:Z},bubbles:!0,composed:!0}))}static styles=U`
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
    .fault { color: var(--err); }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
  `;render(){let X=this.fleet;if(!X||X.inverters.length===0)return G`<div class="empty">No inverters discovered yet.</div>`;return G`
      <table>
        <thead>
          <tr>
            <th>Inverter ID</th><th>Name</th><th>Model</th><th>Status</th>
            <th class="num">Output</th><th class="num">Load</th>
            <th class="num">Grid</th><th class="num">Freq</th>
            <th class="num">Panels</th><th class="num">Faults</th>
          </tr>
        </thead>
        <tbody>
          ${X.inverters.map((Y)=>{let Z=Y.faults?Object.values(Y.faults).filter(Boolean).length:0;return G`<tr>
              <td class="uid">${Y.uid}</td>
              <td>
                <input
                  class="name-in"
                  .value=${this.names?.[Y.uid]??""}
                  placeholder="add a name"
                  @change=${(K)=>this.rename(Y.uid,K)}
                />
              </td>
              <td>${Y.model||"—"}</td>
              <td>
                <span class="dot ${Y.online?"on":"off"}"></span>${Y.online?"online":"offline"}
              </td>
              <td class="num">${C(Y.active_power_w)} / ${Y.nameplate_w} W</td>
              <td class="num">${c(Y.load_pct)}</td>
              <td class="num">${s(Y.grid_v)}</td>
              <td class="num">${KX(Y.freq_hz)}</td>
              <td class="num">${Y.panels?.length??0}</td>
              <td class="num ${Z?"fault":""}">${Z||"—"}</td>
            </tr>`})}
        </tbody>
      </table>
    `}}customElements.define("inverters-view",jY);class zY extends H{static properties={fleet:{attribute:!1}};constructor(){super();this.fleet=null}static styles=U`
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
  `;alarms(){let X=[];for(let Y of this.fleet?.inverters??[]){for(let Z of GX(Y.faults))X.push({uid:Y.uid,model:Y.model,label:Z,severity:"fault"});if(!Y.online)X.push({uid:Y.uid,model:Y.model,label:"Inverter offline",severity:"warning"})}return X}render(){let X=this.alarms();if(X.length===0)return G`<div class="ok"><div class="big">✓ No active alarms</div><div>All inverters reporting healthy.</div></div>`;return G`${X.map((Y)=>G`<div class="row ${Y.severity}">
        <span class="sev">${Y.severity}</span>
        <span class="label">${Y.label} <span style="color:var(--muted)">· ${Y.model||"?"}</span></span>
        <span class="uid">${Y.uid}</span>
      </div>`)}`}}customElements.define("alarms-view",zY);class HY extends H{static properties={events:{attribute:!1}};constructor(){super();this.events=[]}static styles=U`
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
  `;render(){if(!this.events||this.events.length===0)return G`<div class="empty">No events recorded.</div>`;return G`
      <table>
        <thead>
          <tr><th>Time</th><th>Severity</th><th>Event</th><th>Inverter</th><th>Detail</th></tr>
        </thead>
        <tbody>
          ${this.events.map((X)=>G`<tr>
              <td class="time">${BX(X.ts_ms)}</td>
              <td><span class="sev ${YY(X.severity)}">${X.severity}</span></td>
              <td>${NX(X.kind)}</td>
              <td class="uid">${X.inverter_uid||"—"}</td>
              <td class="detail">${X.detail||(X.raw_hex?X.raw_hex:j)}</td>
            </tr>`)}
        </tbody>
      </table>
    `}}customElements.define("events-table",HY);class JY extends H{static properties={events:{state:!0},error:{state:!0},loading:{state:!0}};timer=null;constructor(){super();this.events=[],this.error="",this.loading=!1}static styles=U`
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
  `;connectedCallback(){super.connectedCallback(),this.load(),this.timer=setInterval(()=>void this.load(),15000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async load(){this.loading=!0;try{let X=await _.events({limit:200});this.events=X.events??[],this.error=X.error??""}catch(X){this.error=X.message}finally{this.loading=!1}}render(){return G`
      <div class="bar">
        <span class="count">${this.events.length} event(s)${this.loading?" · refreshing…":""}</span>
        <button @click=${()=>void this.load()}>Refresh</button>
      </div>
      ${this.error?G`<div class="err">⚠ ${this.error}</div>`:j}
      <div class="panel"><events-table .events=${this.events}></events-table></div>
    `}}customElements.define("events-view",JY);class FY extends H{static properties={settings:{attribute:!1}};constructor(){super();this.settings={ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}static styles=U`
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
  `;save=()=>{let X=this.shadowRoot;if(!X)return;let Y=(K)=>(X.querySelector(`#${K}`)?.value??"").trim(),Z={ecu_id:Y("ecu_id"),mac:Y("mac"),pan_override:Y("pan_override"),zigbee_type:Y("zigbee_type")};this.dispatchEvent(new CustomEvent("save",{detail:Z,bubbles:!0,composed:!0}))};render(){let X=this.settings;return G`
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
    `}}customElements.define("settings-form",FY);class UY extends H{static properties={settings:{state:!0},error:{state:!0},notice:{state:!0},loading:{state:!0},saving:{state:!0}};constructor(){super();this.settings=null,this.error="",this.notice="",this.loading=!1,this.saving=!1}static styles=U`
    :host { display: block; }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      max-width: 560px;
    }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){this.loading=!0;try{let X=await _.getSettings();this.settings=X.settings??null,this.error=X.error??""}catch(X){this.error=X.message}finally{this.loading=!1}}onSave=async(X)=>{this.saving=!0,this.notice="",this.error="";try{this.settings=await _.saveSettings(X.detail),this.notice="Settings saved."}catch(Y){this.error=Y.message}finally{this.saving=!1,await this.load()}};render(){return G`
      ${this.notice?G`<div class="banner ok">${this.notice}</div>`:j}
      ${this.error?G`<div class="banner err">⚠ ${this.error}</div>`:j}
      <div class="panel">
        ${this.loading&&!this.settings?G`<div class="loading">Loading…</div>`:G`<settings-form
              .settings=${this.settings??{ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}
              @save=${this.onSave}
            ></settings-form>`}
      </div>
    `}}customElements.define("settings-view",UY);var DX=[{id:"dashboard",label:"Dashboard",icon:"▮▮"},{id:"inverters",label:"Inverters",icon:"⌁"},{id:"alarms",label:"Alarms",icon:"!"},{id:"events",label:"Events",icon:"≣"},{id:"settings",label:"Settings",icon:"⚙"}];class qY extends H{static properties={ready:{state:!0},authed:{state:!0},configured:{state:!0},route:{state:!0},fleet:{state:!0},system:{state:!0},names:{state:!0}};closeSSE=null;sysTimer=null;settingsCache=null;constructor(){super();this.ready=!1,this.authed=!1,this.configured=!0,this.route="dashboard",this.fleet=null,this.system=null,this.names={}}static styles=U`
    :host { display: block; }
    .layout { display: grid; grid-template-columns: 220px 1fr; min-height: 100vh; }
    nav {
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
    @media (max-width: 720px) { .layout { grid-template-columns: 1fr; } nav { display: none; } }
  `;connectedCallback(){super.connectedCallback(),window.addEventListener("hashchange",this.onHash),this.onHash(),this.init()}disconnectedCallback(){super.disconnectedCallback(),window.removeEventListener("hashchange",this.onHash),this.stopStreams()}onHash=()=>{let X=(location.hash.replace(/^#\/?/,"")||"dashboard").split("/")[0];this.route=DX.some((Y)=>Y.id===X)?X:"dashboard"};async init(){try{let X=await _.authStatus();if(this.configured=X.configured,this.authed=X.authenticated,this.authed)this.startStreams()}catch{}finally{this.ready=!0}}onAuthed=async()=>{this.authed=!0,this.startStreams()};logout=async()=>{try{await _.logout()}catch{}this.authed=!1,this.stopStreams(),this.fleet=null,this.system=null};startStreams(){this.stopStreams(),this.closeSSE=nX((Y)=>{this.fleet=Y});let X=()=>_.system().then((Y)=>this.system=Y).catch(()=>{});X(),this.sysTimer=setInterval(X,5000),this.fetchSettings()}async fetchSettings(){try{let X=await _.getSettings();if(X.settings)this.settingsCache=X.settings,this.names=X.settings.inverter_names??{}}catch{}}onRename=async(X)=>{let{uid:Y,name:Z}=X.detail,K=this.settingsCache??{ecu_id:"",mac:"",pan_override:"",zigbee_type:""},Q={...K.inverter_names??{}};if(Z.trim())Q[Y]=Z.trim();else delete Q[Y];let B={...K,inverter_names:Q};try{await _.saveSettings(B),this.settingsCache=B,this.names=Q}catch{}};stopStreams(){if(this.closeSSE?.(),this.closeSSE=null,this.sysTimer)clearInterval(this.sysTimer);this.sysTimer=null}activeView(){switch(this.route){case"inverters":return G`<inverters-view
          .fleet=${this.fleet}
          .names=${this.names}
          @rename=${this.onRename}
        ></inverters-view>`;case"alarms":return G`<alarms-view .fleet=${this.fleet}></alarms-view>`;case"events":return G`<events-view></events-view>`;case"settings":return G`<settings-view></settings-view>`;default:return G`<dashboard-view .fleet=${this.fleet} .system=${this.system} .names=${this.names}></dashboard-view>`}}render(){if(!this.ready)return j;if(!this.authed)return G`<login-view .configured=${this.configured} @authed=${this.onAuthed}></login-view>`;let X=DX.find((Z)=>Z.id===this.route)?.label??"Dashboard",Y=this.system?.invdriver_connected??!1;return G`
      <div class="layout">
        <nav>
          <div class="brand">ECU CONSOLE</div>
          ${DX.map((Z)=>G`<a
              class="item ${this.route===Z.id?"active":""}"
              href="#/${Z.id}"
            ><span class="ic">${Z.icon}</span>${Z.label}</a>`)}
        </nav>
        <main>
          <div class="topbar">
            <h1>${X}</h1>
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
    `}}customElements.define("ecu-app",qY);
