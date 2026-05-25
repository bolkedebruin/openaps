var e=globalThis,FZ=e.ShadowRoot&&(e.ShadyCSS===void 0||e.ShadyCSS.nativeShadow)&&"adoptedStyleSheets"in Document.prototype&&"replace"in CSSStyleSheet.prototype,MZ=Symbol(),bZ=new WeakMap;class kZ{constructor(Z,K,Q){if(this._$cssResult$=!0,Q!==MZ)throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");this.cssText=Z,this._strings=K}get styleSheet(){let Z=this._styleSheet,K=this._strings;if(FZ&&Z===void 0){let Q=K!==void 0&&K.length===1;if(Q)Z=bZ.get(K);if(Z===void 0){if((this._styleSheet=Z=new CSSStyleSheet).replaceSync(this.cssText),Q)bZ.set(K,Z)}}return Z}toString(){return this.cssText}}var w5=(Z)=>{if(Z._$cssResult$===!0)return Z.cssText;else if(typeof Z==="number")return Z;else throw Error(`Value passed to 'css' function must be a 'css' function result: ${Z}. Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.`)},f5=(Z)=>new kZ(typeof Z==="string"?Z:String(Z),void 0,MZ),U=(Z,...K)=>{let Q=Z.length===1?Z[0]:K.reduce(($,B,Y)=>$+w5(B)+Z[Y+1],Z[0]);return new kZ(Q,Z,MZ)},wZ=(Z,K)=>{if(FZ)Z.adoptedStyleSheets=K.map((Q)=>Q instanceof CSSStyleSheet?Q:Q.styleSheet);else for(let Q of K){let $=document.createElement("style"),B=e.litNonce;if(B!==void 0)$.setAttribute("nonce",B);$.textContent=Q.cssText,Z.appendChild($)}},h5=(Z)=>{let K="";for(let Q of Z.cssRules)K+=Q.cssText;return f5(K)},WZ=FZ?(Z)=>Z:(Z)=>Z instanceof CSSStyleSheet?h5(Z):Z;var{is:g5,defineProperty:c5,getOwnPropertyDescriptor:fZ,getOwnPropertyNames:d5,getOwnPropertySymbols:u5,getPrototypeOf:hZ}=Object,v5=!1,V=globalThis;if(v5)V.customElements??=customElements;var C=!0,S,gZ=V.trustedTypes,m5=gZ?gZ.emptyScript:"",dZ=C?V.reactiveElementPolyfillSupportDevMode:V.reactiveElementPolyfillSupport;if(C)V.litIssuedWarnings??=new Set,S=(Z,K)=>{if(K+=` See https://lit.dev/msg/${Z} for more information.`,!V.litIssuedWarnings.has(K)&&!V.litIssuedWarnings.has(Z))console.warn(K),V.litIssuedWarnings.add(K)},queueMicrotask(()=>{if(S("dev-mode","Lit is in dev mode. Not recommended for production!"),V.ShadyDOM?.inUse&&dZ===void 0)S("polyfill-support-missing","Shadow DOM is being polyfilled via `ShadyDOM` but the `polyfill-support` module has not been loaded.")});var p5=C?(Z)=>{if(!V.emitLitDebugLogEvents)return;V.dispatchEvent(new CustomEvent("lit-debug",{detail:Z}))}:void 0,c=(Z,K)=>Z,_Z={toAttribute(Z,K){switch(K){case Boolean:Z=Z?m5:null;break;case Object:case Array:Z=Z==null?Z:JSON.stringify(Z);break}return Z},fromAttribute(Z,K){let Q=Z;switch(K){case Boolean:Q=Z!==null;break;case Number:Q=Z===null?null:Number(Z);break;case Object:case Array:try{Q=JSON.parse(Z)}catch($){Q=null}break}return Q}},uZ=(Z,K)=>!g5(Z,K),cZ={attribute:!0,type:String,converter:_Z,reflect:!1,useDefault:!1,hasChanged:uZ};Symbol.metadata??=Symbol("metadata");V.litPropertyMetadata??=new WeakMap;class D extends HTMLElement{static addInitializer(Z){this.__prepare(),(this._initializers??=[]).push(Z)}static get observedAttributes(){return this.finalize(),this.__attributeToPropertyMap&&[...this.__attributeToPropertyMap.keys()]}static createProperty(Z,K=cZ){if(K.state)K.attribute=!1;if(this.__prepare(),this.prototype.hasOwnProperty(Z))K=Object.create(K),K.wrapped=!0;if(this.elementProperties.set(Z,K),!K.noAccessor){let Q=C?Symbol.for(`${String(Z)} (@property() cache)`):Symbol(),$=this.getPropertyDescriptor(Z,Q,K);if($!==void 0)c5(this.prototype,Z,$)}}static getPropertyDescriptor(Z,K,Q){let{get:$,set:B}=fZ(this.prototype,Z)??{get(){return this[K]},set(Y){this[K]=Y}};if(C&&$==null){if("value"in(fZ(this.prototype,Z)??{}))throw Error(`Field ${JSON.stringify(String(Z))} on ${this.name} was declared as a reactive property but it's actually declared as a value on the prototype. Usually this is due to using @property or @state on a method.`);S("reactive-property-without-getter",`Field ${JSON.stringify(String(Z))} on ${this.name} was declared as a reactive property but it does not have a getter. This will be an error in a future version of Lit.`)}return{get:$,set(Y){let j=$?.call(this);B?.call(this,Y),this.requestUpdate(Z,j,Q)},configurable:!0,enumerable:!0}}static getPropertyOptions(Z){return this.elementProperties.get(Z)??cZ}static __prepare(){if(this.hasOwnProperty(c("elementProperties",this)))return;let Z=hZ(this);if(Z.finalize(),Z._initializers!==void 0)this._initializers=[...Z._initializers];this.elementProperties=new Map(Z.elementProperties)}static finalize(){if(this.hasOwnProperty(c("finalized",this)))return;if(this.finalized=!0,this.__prepare(),this.hasOwnProperty(c("properties",this))){let K=this.properties,Q=[...d5(K),...u5(K)];for(let $ of Q)this.createProperty($,K[$])}let Z=this[Symbol.metadata];if(Z!==null){let K=litPropertyMetadata.get(Z);if(K!==void 0)for(let[Q,$]of K)this.elementProperties.set(Q,$)}this.__attributeToPropertyMap=new Map;for(let[K,Q]of this.elementProperties){let $=this.__attributeNameForProperty(K,Q);if($!==void 0)this.__attributeToPropertyMap.set($,K)}if(this.elementStyles=this.finalizeStyles(this.styles),C){if(this.hasOwnProperty("createProperty"))S("no-override-create-property","Overriding ReactiveElement.createProperty() is deprecated. The override will not be called with standard decorators");if(this.hasOwnProperty("getPropertyDescriptor"))S("no-override-get-property-descriptor","Overriding ReactiveElement.getPropertyDescriptor() is deprecated. The override will not be called with standard decorators")}}static finalizeStyles(Z){let K=[];if(Array.isArray(Z)){let Q=new Set(Z.flat(1/0).reverse());for(let $ of Q)K.unshift(WZ($))}else if(Z!==void 0)K.push(WZ(Z));return K}static __attributeNameForProperty(Z,K){let Q=K.attribute;return Q===!1?void 0:typeof Q==="string"?Q:typeof Z==="string"?Z.toLowerCase():void 0}constructor(){super();this.__instanceProperties=void 0,this.isUpdatePending=!1,this.hasUpdated=!1,this.__reflectingProperty=null,this.__initialize()}__initialize(){this.__updatePromise=new Promise((Z)=>this.enableUpdating=Z),this._$changedProperties=new Map,this.__saveInstanceProperties(),this.requestUpdate(),this.constructor._initializers?.forEach((Z)=>Z(this))}addController(Z){if((this.__controllers??=new Set).add(Z),this.renderRoot!==void 0&&this.isConnected)Z.hostConnected?.()}removeController(Z){this.__controllers?.delete(Z)}__saveInstanceProperties(){let Z=new Map,K=this.constructor.elementProperties;for(let Q of K.keys())if(this.hasOwnProperty(Q))Z.set(Q,this[Q]),delete this[Q];if(Z.size>0)this.__instanceProperties=Z}createRenderRoot(){let Z=this.shadowRoot??this.attachShadow(this.constructor.shadowRootOptions);return wZ(Z,this.constructor.elementStyles),Z}connectedCallback(){this.renderRoot??=this.createRenderRoot(),this.enableUpdating(!0),this.__controllers?.forEach((Z)=>Z.hostConnected?.())}enableUpdating(Z){}disconnectedCallback(){this.__controllers?.forEach((Z)=>Z.hostDisconnected?.())}attributeChangedCallback(Z,K,Q){this._$attributeToProperty(Z,Q)}__propertyToAttribute(Z,K){let $=this.constructor.elementProperties.get(Z),B=this.constructor.__attributeNameForProperty(Z,$);if(B!==void 0&&$.reflect===!0){let j=($.converter?.toAttribute!==void 0?$.converter:_Z).toAttribute(K,$.type);if(C&&this.constructor.enabledWarnings.includes("migration")&&j===void 0)S("undefined-attribute-value",`The attribute value for the ${Z} property is undefined on element ${this.localName}. The attribute will be removed, but in the previous version of \`ReactiveElement\`, the attribute would not have changed.`);if(this.__reflectingProperty=Z,j==null)this.removeAttribute(B);else this.setAttribute(B,j);this.__reflectingProperty=null}}_$attributeToProperty(Z,K){let Q=this.constructor,$=Q.__attributeToPropertyMap.get(Z);if($!==void 0&&this.__reflectingProperty!==$){let B=Q.getPropertyOptions($),Y=typeof B.converter==="function"?{fromAttribute:B.converter}:B.converter?.fromAttribute!==void 0?B.converter:_Z;this.__reflectingProperty=$;let j=Y.fromAttribute(K,B.type);this[$]=j??this.__defaultValues?.get($)??j,this.__reflectingProperty=null}}requestUpdate(Z,K,Q,$=!1,B){if(Z!==void 0){if(C&&Z instanceof Event)S("","The requestUpdate() method was called with an Event as the property name. This is probably a mistake caused by binding this.requestUpdate as an event listener. Instead bind a function that will call it with no arguments: () => this.requestUpdate()");let Y=this.constructor;if($===!1)B=this[Z];if(Q??=Y.getPropertyOptions(Z),(Q.hasChanged??uZ)(B,K)||Q.useDefault&&Q.reflect&&B===this.__defaultValues?.get(Z)&&!this.hasAttribute(Y.__attributeNameForProperty(Z,Q)))this._$changeProperty(Z,K,Q);else return}if(this.isUpdatePending===!1)this.__updatePromise=this.__enqueueUpdate()}_$changeProperty(Z,K,{useDefault:Q,reflect:$,wrapped:B},Y){if(Q&&!(this.__defaultValues??=new Map).has(Z)){if(this.__defaultValues.set(Z,Y??K??this[Z]),B!==!0||Y!==void 0)return}if(!this._$changedProperties.has(Z)){if(!this.hasUpdated&&!Q)K=void 0;this._$changedProperties.set(Z,K)}if($===!0&&this.__reflectingProperty!==Z)(this.__reflectingProperties??=new Set).add(Z)}async __enqueueUpdate(){this.isUpdatePending=!0;try{await this.__updatePromise}catch(K){Promise.reject(K)}let Z=this.scheduleUpdate();if(Z!=null)await Z;return!this.isUpdatePending}scheduleUpdate(){let Z=this.performUpdate();if(C&&this.constructor.enabledWarnings.includes("async-perform-update")&&typeof Z?.then==="function")S("async-perform-update",`Element ${this.localName} returned a Promise from performUpdate(). This behavior is deprecated and will be removed in a future version of ReactiveElement.`);return Z}performUpdate(){if(!this.isUpdatePending)return;if(p5?.({kind:"update"}),!this.hasUpdated){if(this.renderRoot??=this.createRenderRoot(),C){let B=[...this.constructor.elementProperties.keys()].filter((Y)=>this.hasOwnProperty(Y)&&(Y in hZ(this)));if(B.length)throw Error(`The following properties on element ${this.localName} will not trigger updates as expected because they are set using class fields: ${B.join(", ")}. Native class fields and some compiled output will overwrite accessors used for detecting changes. See https://lit.dev/msg/class-field-shadowing for more information.`)}if(this.__instanceProperties){for(let[$,B]of this.__instanceProperties)this[$]=B;this.__instanceProperties=void 0}let Q=this.constructor.elementProperties;if(Q.size>0)for(let[$,B]of Q){let{wrapped:Y}=B,j=this[$];if(Y===!0&&!this._$changedProperties.has($)&&j!==void 0)this._$changeProperty($,void 0,B,j)}}let Z=!1,K=this._$changedProperties;try{if(Z=this.shouldUpdate(K),Z)this.willUpdate(K),this.__controllers?.forEach((Q)=>Q.hostUpdate?.()),this.update(K);else this.__markUpdated()}catch(Q){throw Z=!1,this.__markUpdated(),Q}if(Z)this._$didUpdate(K)}willUpdate(Z){}_$didUpdate(Z){if(this.__controllers?.forEach((K)=>K.hostUpdated?.()),!this.hasUpdated)this.hasUpdated=!0,this.firstUpdated(Z);if(this.updated(Z),C&&this.isUpdatePending&&this.constructor.enabledWarnings.includes("change-in-update"))S("change-in-update",`Element ${this.localName} scheduled an update (generally because a property was set) after an update completed, causing a new update to be scheduled. This is inefficient and should be avoided unless the next update can only be scheduled as a side effect of the previous update.`)}__markUpdated(){this._$changedProperties=new Map,this.isUpdatePending=!1}get updateComplete(){return this.getUpdateComplete()}getUpdateComplete(){return this.__updatePromise}shouldUpdate(Z){return!0}update(Z){this.__reflectingProperties&&=this.__reflectingProperties.forEach((K)=>this.__propertyToAttribute(K,this[K])),this.__markUpdated()}updated(Z){}firstUpdated(Z){}}D.elementStyles=[];D.shadowRootOptions={mode:"open"};D[c("elementProperties",D)]=new Map;D[c("finalized",D)]=new Map;dZ?.({ReactiveElement:D});if(C){D.enabledWarnings=["change-in-update","async-perform-update"];let Z=function(K){if(!K.hasOwnProperty(c("enabledWarnings",K)))K.enabledWarnings=K.enabledWarnings.slice()};D.enableWarning=function(K){if(Z(this),!this.enabledWarnings.includes(K))this.enabledWarnings.push(K)},D.disableWarning=function(K){Z(this);let Q=this.enabledWarnings.indexOf(K);if(Q>=0)this.enabledWarnings.splice(Q,1)}}(V.reactiveElementVersions??=[]).push("2.1.2");if(C&&V.reactiveElementVersions.length>1)queueMicrotask(()=>{S("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var N=globalThis,k=(Z)=>{if(!N.emitLitDebugLogEvents)return;N.dispatchEvent(new CustomEvent("lit-debug",{detail:Z}))},o5=0,p;N.litIssuedWarnings??=new Set,p=(Z,K)=>{if(K+=Z?` See https://lit.dev/msg/${Z} for more information.`:"",!N.litIssuedWarnings.has(K)&&!N.litIssuedWarnings.has(Z))console.warn(K),N.litIssuedWarnings.add(K)},queueMicrotask(()=>{p("dev-mode","Lit is in dev mode. Not recommended for production!")});var P=N.ShadyDOM?.inUse&&N.ShadyDOM?.noPatch===!0?N.ShadyDOM.wrap:(Z)=>Z,ZZ=N.trustedTypes,vZ=ZZ?ZZ.createPolicy("lit-html",{createHTML:(Z)=>Z}):void 0,l5=(Z)=>Z,BZ=(Z,K,Q)=>l5,s5=(Z)=>{if(g!==BZ)throw Error("Attempted to overwrite existing lit-html security policy. setSanitizeDOMValueFactory should be called at most once.");g=Z},r5=()=>{g=BZ},VZ=(Z,K,Q)=>{return g(Z,K,Q)},iZ="$lit$",y=`lit$${Math.random().toFixed(9).slice(2)}$`,aZ="?"+y,i5=`<${aZ}>`,f=document,o=()=>f.createComment(""),l=(Z)=>Z===null||typeof Z!="object"&&typeof Z!="function",CZ=Array.isArray,a5=(Z)=>CZ(Z)||typeof Z?.[Symbol.iterator]==="function",AZ=`[ 	
\f\r]`,n5=`[^ 	
\f\r"'\`<>=]`,t5=`[^\\s"'>=/]`,v=/<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g,mZ=1,IZ=2,e5=3,pZ=/-->/g,oZ=/>/g,x=new RegExp(`>|${AZ}(?:(${t5}+)(${AZ}*=${AZ}*(?:${n5}|("|')|))|$)`,"g"),Z6=0,lZ=1,K6=2,sZ=3,OZ=/'/g,TZ=/"/g,nZ=/^(?:script|style|textarea|title)$/i,Q6=1,KZ=2,QZ=3,DZ=1,$Z=2,$6=3,B6=4,G6=5,NZ=6,Y6=7,RZ=(Z)=>(K,...Q)=>{if(K.some(($)=>$===void 0))console.warn(`Some template strings are undefined.
This is probably caused by illegal octal escape sequences.`);if(Q.some(($)=>$?._$litStatic$))p("",`Static values 'literal' or 'unsafeStatic' cannot be used as values to non-static templates.
Please use the static 'html' tag function. See https://lit.dev/docs/templates/expressions/#static-expressions`);return{["_$litType$"]:Z,strings:K,values:Q}},G=RZ(Q6),R=RZ(KZ),A6=RZ(QZ),h=Symbol.for("lit-noChange"),X=Symbol.for("lit-nothing"),rZ=new WeakMap,w=f.createTreeWalker(f,129),g=BZ;function tZ(Z,K){if(!CZ(Z)||!Z.hasOwnProperty("raw")){let Q="invalid template strings array";throw Q=`
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
`),Error(Q)}return vZ!==void 0?vZ.createHTML(K):K}var j6=(Z,K)=>{let Q=Z.length-1,$=[],B=K===KZ?"<svg>":K===QZ?"<math>":"",Y,j=v;for(let H=0;H<Q;H++){let F=Z[H],J=-1,M,I=0,W;while(I<F.length){if(j.lastIndex=I,W=j.exec(F),W===null)break;if(I=j.lastIndex,j===v){if(W[mZ]==="!--")j=pZ;else if(W[mZ]!==void 0)j=oZ;else if(W[IZ]!==void 0){if(nZ.test(W[IZ]))Y=new RegExp(`</${W[IZ]}`,"g");j=x}else if(W[e5]!==void 0)throw Error("Bindings in tag names are not supported. Please use static templates instead. See https://lit.dev/docs/templates/expressions/#static-expressions")}else if(j===x)if(W[Z6]===">")j=Y??v,J=-1;else if(W[lZ]===void 0)J=-2;else J=j.lastIndex-W[K6].length,M=W[lZ],j=W[sZ]===void 0?x:W[sZ]==='"'?TZ:OZ;else if(j===TZ||j===OZ)j=x;else if(j===pZ||j===oZ)j=v;else j=x,Y=void 0}console.assert(J===-1||j===x||j===OZ||j===TZ,"unexpected parse state B");let T=j===x&&Z[H+1].startsWith("/>")?" ":"";B+=j===v?F+i5:J>=0?($.push(M),F.slice(0,J)+iZ+F.slice(J))+y+T:F+y+(J===-2?H:T)}let z=B+(Z[Q]||"<?>")+(K===KZ?"</svg>":K===QZ?"</math>":"");return[tZ(Z,z),$]};class s{constructor({strings:Z,["_$litType$"]:K},Q){this.parts=[];let $,B=0,Y=0,j=Z.length-1,z=this.parts,[H,F]=j6(Z,K);if(this.el=s.createElement(H,Q),w.currentNode=this.el.content,K===KZ||K===QZ){let J=this.el.content.firstChild;J.replaceWith(...J.childNodes)}while(($=w.nextNode())!==null&&z.length<j){if($.nodeType===1){{let J=$.localName;if(/^(?:textarea|template)$/i.test(J)&&$.innerHTML.includes(y)){let M=`Expressions are not supported inside \`${J}\` elements. See https://lit.dev/msg/expression-in-${J} for more information.`;if(J==="template")throw Error(M);else p("",M)}}if($.hasAttributes()){for(let J of $.getAttributeNames())if(J.endsWith(iZ)){let M=F[Y++],W=$.getAttribute(J).split(y),T=/([.?@])?(.*)/.exec(M);z.push({type:DZ,index:B,name:T[2],strings:W,ctor:T[1]==="."?Z5:T[1]==="?"?K5:T[1]==="@"?Q5:i}),$.removeAttribute(J)}else if(J.startsWith(y))z.push({type:NZ,index:B}),$.removeAttribute(J)}if(nZ.test($.tagName)){let J=$.textContent.split(y),M=J.length-1;if(M>0){$.textContent=ZZ?ZZ.emptyScript:"";for(let I=0;I<M;I++)$.append(J[I],o()),w.nextNode(),z.push({type:$Z,index:++B});$.append(J[M],o())}}}else if($.nodeType===8)if($.data===aZ)z.push({type:$Z,index:B});else{let M=-1;while((M=$.data.indexOf(y,M+1))!==-1)z.push({type:Y6,index:B}),M+=y.length-1}B++}if(F.length!==Y)throw Error('Detected duplicate attribute bindings. This occurs if your template has duplicate attributes on an element tag. For example "<input ?disabled=${true} ?disabled=${false}>" contains a duplicate "disabled" attribute. The error was detected in the following template: \n`'+Z.join("${...}")+"`");k&&k({kind:"template prep",template:this,clonableTemplate:this.el,parts:this.parts,strings:Z})}static createElement(Z,K){let Q=f.createElement("template");return Q.innerHTML=Z,Q}}function d(Z,K,Q=Z,$){if(K===h)return K;let B=$!==void 0?Q.__directives?.[$]:Q.__directive,Y=l(K)?void 0:K._$litDirective$;if(B?.constructor!==Y){if(B?._$notifyDirectiveConnectionChanged?.(!1),Y===void 0)B=void 0;else B=new Y(Z),B._$initialize(Z,Q,$);if($!==void 0)(Q.__directives??=[])[$]=B;else Q.__directive=B}if(B!==void 0)K=d(Z,B._$resolve(Z,K.values),B,$);return K}class eZ{constructor(Z,K){this._$parts=[],this._$disconnectableChildren=void 0,this._$template=Z,this._$parent=K}get parentNode(){return this._$parent.parentNode}get _$isConnected(){return this._$parent._$isConnected}_clone(Z){let{el:{content:K},parts:Q}=this._$template,$=(Z?.creationScope??f).importNode(K,!0);w.currentNode=$;let B=w.nextNode(),Y=0,j=0,z=Q[0];while(z!==void 0){if(Y===z.index){let H;if(z.type===$Z)H=new r(B,B.nextSibling,this,Z);else if(z.type===DZ)H=new z.ctor(B,z.name,z.strings,this,Z);else if(z.type===NZ)H=new $5(B,this,Z);this._$parts.push(H),z=Q[++j]}if(Y!==z?.index)B=w.nextNode(),Y++}return w.currentNode=f,$}_update(Z){let K=0;for(let Q of this._$parts){if(Q!==void 0)if(k&&k({kind:"set part",part:Q,value:Z[K],valueIndex:K,values:Z,templateInstance:this}),Q.strings!==void 0)Q._$setValue(Z,Q,K),K+=Q.strings.length-2;else Q._$setValue(Z[K]);K++}}}class r{get _$isConnected(){return this._$parent?._$isConnected??this.__isConnected}constructor(Z,K,Q,$){this.type=$Z,this._$committedValue=X,this._$disconnectableChildren=void 0,this._$startNode=Z,this._$endNode=K,this._$parent=Q,this.options=$,this.__isConnected=$?.isConnected??!0,this._textSanitizer=void 0}get parentNode(){let Z=P(this._$startNode).parentNode,K=this._$parent;if(K!==void 0&&Z?.nodeType===11)Z=K.parentNode;return Z}get startNode(){return this._$startNode}get endNode(){return this._$endNode}_$setValue(Z,K=this){if(this.parentNode===null)throw Error("This `ChildPart` has no `parentNode` and therefore cannot accept a value. This likely means the element containing the part was manipulated in an unsupported way outside of Lit's control such that the part's marker nodes were ejected from DOM. For example, setting the element's `innerHTML` or `textContent` can do this.");if(Z=d(this,Z,K),l(Z)){if(Z===X||Z==null||Z===""){if(this._$committedValue!==X)k&&k({kind:"commit nothing to child",start:this._$startNode,end:this._$endNode,parent:this._$parent,options:this.options}),this._$clear();this._$committedValue=X}else if(Z!==this._$committedValue&&Z!==h)this._commitText(Z)}else if(Z._$litType$!==void 0)this._commitTemplateResult(Z);else if(Z.nodeType!==void 0){if(this.options?.host===Z){this._commitText("[probable mistake: rendered a template's host in itself (commonly caused by writing ${this} in a template]"),console.warn("Attempted to render the template host",Z,"inside itself. This is almost always a mistake, and in dev mode ","we render some warning text. In production however, we'll ","render it, which will usually result in an error, and sometimes ","in the element disappearing from the DOM.");return}this._commitNode(Z)}else if(a5(Z))this._commitIterable(Z);else this._commitText(Z)}_insert(Z){return P(P(this._$startNode).parentNode).insertBefore(Z,this._$endNode)}_commitNode(Z){if(this._$committedValue!==Z){if(this._$clear(),g!==BZ){let K=this._$startNode.parentNode?.nodeName;if(K==="STYLE"||K==="SCRIPT"){let Q="Forbidden";if(K==="STYLE")Q="Lit does not support binding inside style nodes. This is a security risk, as style injection attacks can exfiltrate data and spoof UIs. Consider instead using css`...` literals to compose styles, and do dynamic styling with css custom properties, ::parts, <slot>s, and by mutating the DOM rather than stylesheets.";else Q="Lit does not support binding inside script nodes. This is a security risk, as it could allow arbitrary code execution.";throw Error(Q)}}k&&k({kind:"commit node",start:this._$startNode,parent:this._$parent,value:Z,options:this.options}),this._$committedValue=this._insert(Z)}}_commitText(Z){if(this._$committedValue!==X&&l(this._$committedValue)){let K=P(this._$startNode).nextSibling;if(this._textSanitizer===void 0)this._textSanitizer=VZ(K,"data","property");Z=this._textSanitizer(Z),k&&k({kind:"commit text",node:K,value:Z,options:this.options}),K.data=Z}else{let K=f.createTextNode("");if(this._commitNode(K),this._textSanitizer===void 0)this._textSanitizer=VZ(K,"data","property");Z=this._textSanitizer(Z),k&&k({kind:"commit text",node:K,value:Z,options:this.options}),K.data=Z}this._$committedValue=Z}_commitTemplateResult(Z){let{values:K,["_$litType$"]:Q}=Z,$=typeof Q==="number"?this._$getTemplate(Z):(Q.el===void 0&&(Q.el=s.createElement(tZ(Q.h,Q.h[0]),this.options)),Q);if(this._$committedValue?._$template===$)k&&k({kind:"template updating",template:$,instance:this._$committedValue,parts:this._$committedValue._$parts,options:this.options,values:K}),this._$committedValue._update(K);else{let B=new eZ($,this),Y=B._clone(this.options);k&&k({kind:"template instantiated",template:$,instance:B,parts:B._$parts,options:this.options,fragment:Y,values:K}),B._update(K),k&&k({kind:"template instantiated and updated",template:$,instance:B,parts:B._$parts,options:this.options,fragment:Y,values:K}),this._commitNode(Y),this._$committedValue=B}}_$getTemplate(Z){let K=rZ.get(Z.strings);if(K===void 0)rZ.set(Z.strings,K=new s(Z));return K}_commitIterable(Z){if(!CZ(this._$committedValue))this._$committedValue=[],this._$clear();let K=this._$committedValue,Q=0,$;for(let B of Z){if(Q===K.length)K.push($=new r(this._insert(o()),this._insert(o()),this,this.options));else $=K[Q];$._$setValue(B),Q++}if(Q<K.length)this._$clear($&&P($._$endNode).nextSibling,Q),K.length=Q}_$clear(Z=P(this._$startNode).nextSibling,K){this._$notifyConnectionChanged?.(!1,!0,K);while(Z!==this._$endNode){let Q=P(Z).nextSibling;P(Z).remove(),Z=Q}}setConnected(Z){if(this._$parent===void 0)this.__isConnected=Z,this._$notifyConnectionChanged?.(Z);else throw Error("part.setConnected() may only be called on a RootPart returned from render().")}}class i{get tagName(){return this.element.tagName}get _$isConnected(){return this._$parent._$isConnected}constructor(Z,K,Q,$,B){if(this.type=DZ,this._$committedValue=X,this._$disconnectableChildren=void 0,this.element=Z,this.name=K,this._$parent=$,this.options=B,Q.length>2||Q[0]!==""||Q[1]!=="")this._$committedValue=Array(Q.length-1).fill(new String),this.strings=Q;else this._$committedValue=X;this._sanitizer=void 0}_$setValue(Z,K=this,Q,$){let B=this.strings,Y=!1;if(B===void 0){if(Z=d(this,Z,K,0),Y=!l(Z)||Z!==this._$committedValue&&Z!==h,Y)this._$committedValue=Z}else{let j=Z;Z=B[0];let z,H;for(z=0;z<B.length-1;z++){if(H=d(this,j[Q+z],K,z),H===h)H=this._$committedValue[z];if(Y||=!l(H)||H!==this._$committedValue[z],H===X)Z=X;else if(Z!==X)Z+=(H??"")+B[z+1];this._$committedValue[z]=H}}if(Y&&!$)this._commitValue(Z)}_commitValue(Z){if(Z===X)P(this.element).removeAttribute(this.name);else{if(this._sanitizer===void 0)this._sanitizer=g(this.element,this.name,"attribute");Z=this._sanitizer(Z??""),k&&k({kind:"commit attribute",element:this.element,name:this.name,value:Z,options:this.options}),P(this.element).setAttribute(this.name,Z??"")}}}class Z5 extends i{constructor(){super(...arguments);this.type=$6}_commitValue(Z){if(this._sanitizer===void 0)this._sanitizer=g(this.element,this.name,"property");Z=this._sanitizer(Z),k&&k({kind:"commit property",element:this.element,name:this.name,value:Z,options:this.options}),this.element[this.name]=Z===X?void 0:Z}}class K5 extends i{constructor(){super(...arguments);this.type=B6}_commitValue(Z){k&&k({kind:"commit boolean attribute",element:this.element,name:this.name,value:!!(Z&&Z!==X),options:this.options}),P(this.element).toggleAttribute(this.name,!!Z&&Z!==X)}}class Q5 extends i{constructor(Z,K,Q,$,B){super(Z,K,Q,$,B);if(this.type=G6,this.strings!==void 0)throw Error(`A \`<${Z.localName}>\` has a \`@${K}=...\` listener with invalid content. Event listeners in templates must have exactly one expression and no surrounding text.`)}_$setValue(Z,K=this){if(Z=d(this,Z,K,0)??X,Z===h)return;let Q=this._$committedValue,$=Z===X&&Q!==X||Z.capture!==Q.capture||Z.once!==Q.once||Z.passive!==Q.passive,B=Z!==X&&(Q===X||$);if(k&&k({kind:"commit event listener",element:this.element,name:this.name,value:Z,options:this.options,removeListener:$,addListener:B,oldListener:Q}),$)this.element.removeEventListener(this.name,this,Q);if(B)this.element.addEventListener(this.name,this,Z);this._$committedValue=Z}handleEvent(Z){if(typeof this._$committedValue==="function")this._$committedValue.call(this.options?.host??this.element,Z);else this._$committedValue.handleEvent(Z)}}class $5{constructor(Z,K,Q){this.element=Z,this.type=NZ,this._$disconnectableChildren=void 0,this._$parent=K,this.options=Q}get _$isConnected(){return this._$parent._$isConnected}_$setValue(Z){k&&k({kind:"commit to element binding",element:this.element,value:Z,options:this.options}),d(this,Z)}}var X6=N.litHtmlPolyfillSupportDevMode;X6?.(s,r);(N.litHtmlVersions??=[]).push("3.3.3");if(N.litHtmlVersions.length>1)queueMicrotask(()=>{p("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var m=(Z,K,Q)=>{if(K==null)throw TypeError(`The container to render into may not be ${K}`);let $=o5++,B=Q?.renderBefore??K,Y=B._$litPart$;if(k&&k({kind:"begin render",id:$,value:Z,container:K,options:Q,part:Y}),Y===void 0){let j=Q?.renderBefore??null;B._$litPart$=Y=new r(K.insertBefore(o(),j),j,void 0,Q??{})}return Y._$setValue(Z),k&&k({kind:"end render",id:$,value:Z,container:K,options:Q,part:Y}),Y};m.setSanitizer=s5,m.createSanitizer=VZ,m._testOnlyClearSanitizerFactoryDoNotCallOrElse=r5;var z6=(Z,K)=>Z,LZ=!0,E=globalThis,B5;if(LZ)E.litIssuedWarnings??=new Set,B5=(Z,K)=>{if(K+=` See https://lit.dev/msg/${Z} for more information.`,!E.litIssuedWarnings.has(K)&&!E.litIssuedWarnings.has(Z))console.warn(K),E.litIssuedWarnings.add(K)};class q extends D{constructor(){super(...arguments);this.renderOptions={host:this},this.__childPart=void 0}createRenderRoot(){let Z=super.createRenderRoot();return this.renderOptions.renderBefore??=Z.firstChild,Z}update(Z){let K=this.render();if(!this.hasUpdated)this.renderOptions.isConnected=this.isConnected;super.update(Z),this.__childPart=m(K,this.renderRoot,this.renderOptions)}connectedCallback(){super.connectedCallback(),this.__childPart?.setConnected(!0)}disconnectedCallback(){super.disconnectedCallback(),this.__childPart?.setConnected(!1)}render(){return h}}q._$litElement$=!0;q[z6("finalized",q)]=!0;E.litElementHydrateSupport?.({LitElement:q});var J6=LZ?E.litElementPolyfillSupportDevMode:E.litElementPolyfillSupport;J6?.({LitElement:q});(E.litElementVersions??=[]).push("4.2.2");if(LZ&&E.litElementVersions.length>1)queueMicrotask(()=>{B5("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});async function b(Z){let K=await fetch(Z,{credentials:"same-origin"});if(!K.ok)throw Error(`${Z}: ${K.status}`);return await K.json()}async function GZ(Z,K){let Q=await fetch(Z,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(K)});if(!Q.ok){let $=await Q.text();throw Error($.trim()||`${Z}: ${Q.status}`)}}async function G5(Z,K){let Q=await fetch(Z,{method:"PUT",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(K)});if(!Q.ok){let $=await Q.text();throw Error($.trim()||`${Z}: ${Q.status}`)}return await Q.json()}async function q6(Z,K){let Q=await fetch(Z,{method:"DELETE",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(K)});if(!Q.ok){let $=await Q.text();throw Error($.trim()||`${Z}: ${Q.status}`)}return await Q.json()}var A={authStatus:()=>b("/api/auth/status"),setup:(Z)=>GZ("/api/auth/setup",{password:Z}),login:(Z)=>GZ("/api/auth/login",{password:Z}),logout:()=>GZ("/api/auth/logout",{}),fleet:()=>b("/api/fleet"),system:()=>b("/api/system"),history:()=>b("/api/history"),events:(Z={})=>{let K=new URLSearchParams;if(Z.since_ms)K.set("since_ms",String(Z.since_ms));if(Z.kind)K.set("kind",Z.kind);if(Z.severity)K.set("severity",Z.severity);if(Z.inverter_uid)K.set("inverter_uid",Z.inverter_uid);if(Z.limit)K.set("limit",String(Z.limit));let Q=K.toString();return b("/api/events"+(Q?`?${Q}`:""))},getSettings:async()=>{let Z=await b("/api/settings");if(Z.error)return{error:Z.error};return{settings:{ecu_id:Z.ecu_id,mac:Z.mac,pan_override:Z.pan_override,zigbee_type:Z.zigbee_type,inverter_names:Z.inverter_names??{}}}},saveSettings:(Z)=>G5("/api/settings",Z),profiles:()=>b("/api/profiles"),overlays:()=>b("/api/overlays"),selectBase:(Z)=>GZ("/api/profiles/base",{id:Z}),saveOverlay:(Z)=>G5("/api/profiles/overlay",Z),deleteOverlay:(Z,K)=>q6("/api/profiles/overlay",{id:Z,uids:K})};function Y5(Z,K){let Q=new EventSource("/api/stream");return Q.addEventListener("fleet",($)=>{try{Z(JSON.parse($.data))}catch{}}),Q.onerror=()=>K?.(),()=>Q.close()}class j5 extends q{static properties={configured:{type:Boolean},error:{state:!0},busy:{state:!0}};constructor(){super();this.configured=!0,this.error="",this.busy=!1}static styles=U`
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
  `;async submit(Z){Z.preventDefault();let Q=this.renderRoot.querySelector("input")?.value??"";this.busy=!0,this.error="";try{if(this.configured)await A.login(Q);else await A.setup(Q);this.dispatchEvent(new CustomEvent("authed",{bubbles:!0,composed:!0}))}catch($){this.error=$.message||"failed"}finally{this.busy=!1}}render(){return G`
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
    `}}customElements.define("login-view",j5);function L(Z){if(!Number.isFinite(Z))return"—";if(Math.abs(Z)>=1000)return`${(Z/1000).toFixed(2)} kW`;return`${Math.round(Z)} W`}function a(Z){if(!Number.isFinite(Z))return"—";let K=Math.abs(Z);if(K>=1e6)return`${(Z/1e6).toFixed(2)} MWh`;if(K>=1000)return`${(Z/1000).toFixed(2)} kWh`;return`${Math.round(Z)} Wh`}function u(Z){return Number.isFinite(Z)?`${Z.toFixed(0)}%`:"—"}function n(Z){return Z>0?`${Z.toFixed(1)} V`:"—"}function YZ(Z){return Z>0?`${Z.toFixed(2)} Hz`:"—"}function X5(Z){return Number.isFinite(Z)?`${Z.toFixed(2)} A`:"—"}function jZ(Z){if(!(Z>0))return"idle";if(Z<40)return"low";if(Z<85)return"mid";return"high"}function z5(Z){if(!Number.isFinite(Z)||Z<0)return"—";if(Z<60)return`${Math.round(Z)}s ago`;if(Z<3600)return`${Math.round(Z/60)}m ago`;return`${Math.round(Z/3600)}h ago`}function SZ(Z){return Z.replace(/_/g," ").replace(/\b\w/g,(K)=>K.toUpperCase())}function XZ(Z){if(!Z)return[];return Object.keys(Z).filter((K)=>Z[K]).map(SZ)}function zZ(Z){if(!Z)return"—";return new Date(Z).toLocaleString(void 0,{hour12:!1})}function J5(Z){let K=(Z||"").toLowerCase();if(K==="error"||K==="critical"||K==="crit"||K==="fault")return"err";if(K==="warn"||K==="warning")return"warn";return"info"}class q5 extends q{static properties={power:{type:Number},cap:{type:Number}};constructor(){super();this.power=0,this.cap=0}static styles=U`
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
  `;pct(){if(!(this.cap>0))return 0;return Math.max(0,Math.min(100,this.power/this.cap*100))}render(){let Z=this.pct(),K=jZ(Z),Q=90,$=Math.PI*90,B=$*(1-Z/100);return G`
      <div class="wrap">
        <svg viewBox="0 0 200 120" role="img" aria-label="fleet output gauge">
          <path
            class="track"
            d="M10 110 A 90 90 0 0 1 190 110"
            fill="none"
            stroke-width="14"
          />
          <path
            class="arc ${K}"
            d="M10 110 A 90 90 0 0 1 190 110"
            fill="none"
            stroke-width="14"
            stroke-dasharray="${$}"
            stroke-dashoffset="${B}"
          />
        </svg>
        <div class="center">
          <div class="big">${L(this.power)}</div>
          <div class="sub">${u(Z)} of ${L(this.cap)}</div>
        </div>
      </div>
    `}}customElements.define("fleet-gauge",q5);class H5 extends q{static properties={label:{type:String},value:{type:String},sub:{type:String}};constructor(){super();this.label="",this.value="",this.sub=""}static styles=U`
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
    `}}customElements.define("stat-card",H5);class U5 extends q{static properties={inverter:{attribute:!1},name:{type:String},profile:{type:String}};constructor(){super();this.name="",this.profile=""}static styles=U`
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
  `;render(){let Z=this.inverter;if(!Z)return X;let K=jZ(Z.load_pct),Q=XZ(Z.faults),$=Math.max(0,Math.min(100,Z.load_pct));return G`
      <div class="head">
        <div>
          <div class="model">${this.name||Z.model||"unknown"}</div>
          <div class="uid">${this.name?`${Z.model} · ${Z.uid}`:Z.uid}</div>
          ${this.profile?G`<div class="profile" title="Local Site profile active">⚙ ${this.profile}</div>`:X}
        </div>
        <div class="state">
          <span class="dot ${Z.online?"on":"off"}"></span>
          ${Z.online?"online":"offline"} · ${z5(Z.age_s)}
        </div>
      </div>

      <div class="power">
        <span class="pw">${L(Z.active_power_w)}</span>
        <span class="cap">/ ${Z.nameplate_w} W · ${u(Z.load_pct)}</span>
      </div>
      <div class="bar"><div class="fill ${K}" style="width:${$}%"></div></div>

      <div class="metrics">
        <div class="metric"><div class="k">Grid</div><div class="v">${n(Z.grid_v)}</div></div>
        <div class="metric"><div class="k">Freq</div><div class="v">${YZ(Z.freq_hz)}</div></div>
        <div class="metric"><div class="k">RSSI / LQI</div><div class="v">${Z.rssi} / ${Z.lqi}</div></div>
      </div>

      ${Z.panels?.length?G`<div class="panels">
            ${Z.panels.map((B)=>G`<div class="panel">
                <div class="pi">DC ${B.index+1}</div>
                <div class="pw">${L(B.w)}</div>
                <div>${n(B.dc_v)} · ${X5(B.dc_a)}</div>
              </div>`)}
          </div>`:X}

      ${Q.length?G`<div class="chips">
            ${Q.map((B)=>G`<span class="chip">${B}</span>`)}
          </div>`:X}
    `}}customElements.define("inverter-card",U5);class F5 extends q{static properties={system:{attribute:!1}};constructor(){super();this.system=null}static styles=U`
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
  `;idRow(Z,K){return K?G`<div class="k">${Z}</div><div class="v">${K}</div>`:X}clients(){let Z=new Map;for(let K of this.system?.peers??[]){let Q=Z.get(K.backend)??{backend:K.backend,version:K.version,controller:!1,conns:0};if(Q.conns++,Q.controller=Q.controller||K.controller,K.version)Q.version=K.version;Z.set(K.backend,Q)}return[...Z.values()].sort((K,Q)=>K.backend.localeCompare(Q.backend))}render(){let Z=this.system,K=Z?.ecu,Q=this.clients(),$=!!(K&&(K.ecu_id||K.hostname));return G`
      ${$?G`<div class="id">
            ${this.idRow("ECU ID",K.ecu_id)}
            ${this.idRow("Host",K.hostname)}
          </div>`:X}

      <div class="peers">
        ${Q.length?Q.map((B)=>G`<div class="peer">
                <span class="dot on"></span>
                <span class="name">${B.backend||"(unnamed)"}</span>
                ${B.controller?G`<span class="role ctl">ctrl</span>`:X}
                ${B.conns>1?G`<span class="role">${B.conns} conns</span>`:X}
                <span class="ver">${B.version||""}</span>
              </div>`):G`<div class="empty">No peers connected.</div>`}
      </div>

      ${Z?.status_error?G`<div class="warn">⚠ ${Z.status_error}</div>`:X}
    `}}customElements.define("ecu-clients-card",F5);function H6(Z,K,Q){if(Z.length<2)return{line:"",area:"",max:0};let $=Z[0].t,B=Math.max(1,Z[Z.length-1].t-$),Y=Math.max(1,...Z.map((M)=>M.w)),j=(M)=>[(M.t-$)/B*K,Q-M.w/Y*Q],z="";for(let M=0;M<Z.length;M++){let[I,W]=j(Z[M]);z+=`${M===0?"M":"L"}${I.toFixed(1)} ${W.toFixed(1)} `}let[H]=j(Z[0]),[F]=j(Z[Z.length-1]),J=`${z}L${F.toFixed(1)} ${Q} L${H.toFixed(1)} ${Q} Z`;return{line:z.trim(),area:J,max:Y}}var JZ=600,t=160;class M5 extends q{static properties={points:{attribute:!1},hoverIdx:{state:!0}};constructor(){super();this.points=[],this.hoverIdx=-1}static styles=U`
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
  `;onMove=(Z)=>{let K=this.points.length;if(K<2)return;let $=Z.currentTarget.clientWidth||1,B=Math.min(1,Math.max(0,Z.offsetX/$));this.hoverIdx=Math.round(B*(K-1))};onLeave=()=>{this.hoverIdx=-1};render(){let Z=this.points??[];if(Z.length<2)return G`<div class="empty">Collecting power history…</div>`;let{line:K,area:Q,max:$}=H6(Z,JZ,t),B=Z[Z.length-1].w,Y=this.hoverIdx,j=Y>=0&&Y<Z.length,z=Z[0].t,H=Math.max(1,Z[Z.length-1].t-z),F=j?(Z[Y].t-z)/H*JZ:0,J=j?t-Z[Y].w/$*t:0;return G`
      <div class="wrap">
        <svg
          viewBox="0 0 ${JZ} ${t}"
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
          ${R`<path class="area" d=${Q} />`}
          ${R`<path class="line" d=${K} />`}
          ${j?R`<line class="cross" x1=${F} y1="0" x2=${F} y2=${t} /><circle class="cursor" cx=${F} cy=${J} r="3.5" />`:X}
        </svg>
        ${j?G`<div class="tip" style="left:${F/JZ*100}%; top:${J}px">
              <span class="w">${L(Z[Y].w)}</span>
              <span class="t">· ${zZ(Z[Y].t)}</span>
            </div>`:X}
      </div>
      <div class="labels">
        <span>now <span class="cur">${L(B)}</span></span>
        <span>peak ${L($)}</span>
      </div>
    `}}customElements.define("power-chart",M5);class k5 extends q{static properties={fleet:{attribute:!1},system:{attribute:!1},names:{attribute:!1},profiles:{attribute:!1},history:{state:!0}};timer=null;constructor(){super();this.fleet=null,this.system=null,this.names={},this.profiles={},this.history=[]}connectedCallback(){super.connectedCallback(),this.loadHistory(),this.timer=setInterval(()=>void this.loadHistory(),60000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async loadHistory(){try{this.history=await A.history()}catch{}}chartPoints(){if(!this.fleet)return this.history;return[...this.history,{t:Date.now(),w:this.fleet.active_power_w}]}static styles=U`
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
  `;render(){let Z=this.fleet;if(!Z)return G`<div class="empty">Waiting for inv-driver…</div>`;return G`
      <div class="grid">
        <div class="panel">
          <h2>Array output</h2>
          <fleet-gauge .power=${Z.active_power_w} .cap=${Z.nameplate_total_w}></fleet-gauge>
          <div class="online">${Z.online_count} / ${Z.inverter_count} inverters online</div>
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
        <stat-card label="Today" value=${a(Z.today_wh)}></stat-card>
        <stat-card label="This month" value=${a(Z.month_wh)}></stat-card>
        <stat-card label="This year" value=${a(Z.year_wh)}></stat-card>
        <stat-card label="Lifetime" value=${a(Z.lifetime_wh)}></stat-card>
      </div>

      <h2>Inverters</h2>
      ${Z.inverters.length?G`<div class="cards">
            ${Z.inverters.map((K)=>G`<inverter-card
                .inverter=${K}
                .name=${this.names?.[K.uid]??""}
                .profile=${this.profiles?.[K.uid]??""}
              ></inverter-card>`)}
          </div>`:G`<div class="empty">No inverters discovered yet.</div>`}
      ${X}
    `}}customElements.define("dashboard-view",k5);class W5 extends q{static properties={fleet:{attribute:!1},names:{attribute:!1}};constructor(){super();this.fleet=null,this.names={}}rename(Z,K){let Q=K.target.value;this.dispatchEvent(new CustomEvent("rename",{detail:{uid:Z,name:Q},bubbles:!0,composed:!0}))}static styles=U`
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
  `;render(){let Z=this.fleet;if(!Z||Z.inverters.length===0)return G`<div class="empty">No inverters discovered yet.</div>`;return G`
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
          ${Z.inverters.map((K)=>{let Q=K.faults?Object.values(K.faults).filter(Boolean).length:0;return G`<tr>
              <td class="uid">${K.uid}</td>
              <td>
                <input
                  class="name-in"
                  .value=${this.names?.[K.uid]??""}
                  placeholder="add a name"
                  @change=${($)=>this.rename(K.uid,$)}
                />
              </td>
              <td>${K.model||"—"}</td>
              <td class="fw">${K.sw_version||"—"}</td>
              <td>
                <span class="dot ${K.online?"on":"off"}"></span>${K.online?"online":"offline"}
              </td>
              <td class="num">${L(K.active_power_w)} / ${K.nameplate_w} W</td>
              <td class="num">${u(K.load_pct)}</td>
              <td class="num">${n(K.grid_v)}</td>
              <td class="num">${YZ(K.freq_hz)}</td>
              <td class="num">${K.panels?.length??0}</td>
              <td class="num ${Q?"fault":""}">${Q||"—"}</td>
            </tr>`})}
        </tbody>
      </table>
    `}}customElements.define("inverters-view",W5);class _5 extends q{static properties={fleet:{attribute:!1}};constructor(){super();this.fleet=null}static styles=U`
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
  `;alarms(){let Z=[];for(let K of this.fleet?.inverters??[]){for(let Q of XZ(K.faults))Z.push({uid:K.uid,model:K.model,label:Q,severity:"fault"});if(!K.online)Z.push({uid:K.uid,model:K.model,label:"Inverter offline",severity:"warning"})}return Z}render(){let Z=this.alarms();if(Z.length===0)return G`<div class="ok"><div class="big">✓ No active alarms</div><div>All inverters reporting healthy.</div></div>`;return G`${Z.map((K)=>G`<div class="row ${K.severity}">
        <span class="sev">${K.severity}</span>
        <span class="label">${K.label} <span style="color:var(--muted)">· ${K.model||"?"}</span></span>
        <span class="uid">${K.uid}</span>
      </div>`)}`}}customElements.define("alarms-view",_5);class A5 extends q{static properties={events:{attribute:!1}};constructor(){super();this.events=[]}static styles=U`
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
          ${this.events.map((Z)=>G`<tr>
              <td class="time">${zZ(Z.ts_ms)}</td>
              <td><span class="sev ${J5(Z.severity)}">${Z.severity}</span></td>
              <td>${SZ(Z.kind)}</td>
              <td class="uid">${Z.inverter_uid||"—"}</td>
              <td class="detail">${Z.detail||(Z.raw_hex?Z.raw_hex:X)}</td>
            </tr>`)}
        </tbody>
      </table>
    `}}customElements.define("events-table",A5);class I5 extends q{static properties={events:{state:!0},error:{state:!0},loading:{state:!0}};timer=null;constructor(){super();this.events=[],this.error="",this.loading=!1}static styles=U`
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
  `;connectedCallback(){super.connectedCallback(),this.load(),this.timer=setInterval(()=>void this.load(),15000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async load(){this.loading=!0;try{let Z=await A.events({limit:200});this.events=Z.events??[],this.error=Z.error??""}catch(Z){this.error=Z.message}finally{this.loading=!1}}render(){return G`
      <div class="bar">
        <span class="count">${this.events.length} event(s)${this.loading?" · refreshing…":""}</span>
        <button @click=${()=>void this.load()}>Refresh</button>
      </div>
      ${this.error?G`<div class="err">⚠ ${this.error}</div>`:X}
      <div class="panel"><events-table .events=${this.events}></events-table></div>
    `}}customElements.define("events-view",I5);class O5 extends q{static properties={profiles:{attribute:!1},activeBase:{attribute:!1},reconcilerReady:{attribute:!1},busy:{attribute:!1},selected:{state:!0}};constructor(){super();this.profiles=[],this.activeBase="",this.reconcilerReady=!0,this.busy=!1,this.selected=""}static styles=U`
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
  `;onChange=(Z)=>{this.selected=Z.target.value};apply=()=>{let Z=this.effectiveSelected();if(!Z||Z===this.activeBase)return;this.dispatchEvent(new CustomEvent("apply",{detail:Z,bubbles:!0,composed:!0}))};effectiveSelected(){return this.selected||this.activeBase}labelFor(Z){let K=[`${Z.vnom_v} V`];if(Z.source_ref)K.push(Z.source_ref);return K.push(`${Z.point_count} pts`),`${Z.id} — ${K.join(" · ")}`}render(){let Z=this.effectiveSelected(),K=this.profiles.find(($)=>$.id===this.activeBase),Q=!this.busy&&this.reconcilerReady&&Z!==""&&Z!==this.activeBase;return G`
      <div class="grid">
        <div class="active">
          <span class="muted">Active profile:</span>
          ${this.activeBase?G` <strong>${this.activeBase}</strong>${K?G` <span class="muted">(${K.vnom_v} V · ${K.point_count} pts)</span>`:X}`:G` <span class="none">none selected</span>`}
        </div>

        <label>
          Base profile
          <select id="profile" .value=${Z} @change=${this.onChange} ?disabled=${this.busy}>
            ${this.activeBase?X:G`<option value="" disabled selected>Select a profile…</option>`}
            ${this.profiles.map(($)=>G`<option value=${$.id} ?selected=${$.id===Z}>${this.labelFor($)}</option>`)}
          </select>
        </label>

        <div class="actions">
          <button class="apply" @click=${this.apply} ?disabled=${!Q}>
            ${this.busy?"Applying…":"Apply"}
          </button>
          ${!this.reconcilerReady?G`<span class="hint">reconciler not ready</span>`:Z&&Z!==this.activeBase?G`<span class="hint">applies to all inverters</span>`:X}
        </div>
      </div>
    `}}customElements.define("grid-profile-form",O5);var T5={AC:{label:"Undervoltage trip — stage 2",desc:"Disconnect when AC voltage drops to this lower-stage level."},AQ:{label:"Undervoltage trip — deep",desc:"Disconnect quickly when voltage falls this far below nominal."},AH:{label:"Undervoltage trip — fast",desc:"Fast disconnect on a severe undervoltage."},AD:{label:"Overvoltage trip — slow",desc:"Disconnect when AC voltage rises above this (slower stage)."},AY:{label:"Overvoltage trip — slow (stage 2)",desc:"Second slower overvoltage disconnect threshold."},AB:{label:"10-minute mean overvoltage",desc:"Trips if the 10-minute average voltage exceeds this (EN 50549 sustained-overvoltage limit)."},AI:{label:"Overvoltage trip — fast",desc:"Fast disconnect on a severe overvoltage."},AE:{label:"Underfrequency trip — slow",desc:"Disconnect when grid frequency falls below this (slower stage)."},AJ:{label:"Underfrequency trip — fast",desc:"Fast disconnect on a severe underfrequency."},AF:{label:"Overfrequency trip — slow",desc:"Disconnect when grid frequency rises above this (slower stage)."},AK:{label:"Overfrequency trip — fast",desc:"Fast disconnect on a severe overfrequency."},BB:{label:"Undervoltage 1 — clearance time",desc:"How long the undervoltage condition must persist before tripping."},BD:{label:"Undervoltage 2 — clearance time",desc:"Clearance delay for the second undervoltage stage."},BC:{label:"Overvoltage 1 — clearance time",desc:"How long the overvoltage condition must persist before tripping."},BE:{label:"Overvoltage 2 — clearance time",desc:"Clearance delay for the second overvoltage stage."},BH:{label:"Underfrequency 1 — clearance time",desc:"Clearance delay for the first underfrequency stage."},BJ:{label:"Underfrequency 2 — clearance time",desc:"Clearance delay for the second underfrequency stage."},BI:{label:"Overfrequency 1 — clearance time",desc:"Clearance delay for the first overfrequency stage."},BK:{label:"Overfrequency 2 — clearance time",desc:"Clearance delay for the second overfrequency stage."},BN:{label:"Enter-service voltage — lower",desc:"Voltage must be above this before the inverter reconnects."},BO:{label:"Enter-service voltage — upper",desc:"Voltage must be below this before the inverter reconnects."},BP:{label:"Enter-service frequency — lower",desc:"Frequency must be above this before the inverter reconnects."},BQ:{label:"Enter-service frequency — upper",desc:"Frequency must be below this before the inverter reconnects."},AG:{label:"Grid-recovery delay",desc:"Wait time after the grid is healthy before reconnecting."},AS:{label:"Power ramp time",desc:"Time taken to ramp output back up after reconnecting."},CV:{label:"Curtailment enable (droop)",desc:"Enables the over-frequency droop power reduction (0 = off, 1 = on)."},CA:{label:"Curtailment start (droop deadband)",desc:"Over-frequency droop: power reduction begins at this frequency (deadband end)."},DD:{label:"Curtailment slope (droop)",desc:"Over-frequency droop gradient: % of rated power reduced per Hz above the start."},CG:{label:"Curtailment response time (droop)",desc:"Filter/response time of the droop control loop."},DH:{label:"Under-freq curve — low",desc:"Legacy frequency-Watt curve: lower frequency point of the under-frequency response."},DI:{label:"Under-freq curve — high",desc:"Legacy frequency-Watt curve: upper frequency point of the under-frequency response."},CB:{label:"Over-freq curve — start",desc:"Legacy frequency-Watt curve: over-frequency power reduction begins at this frequency."},CC:{label:"Over-freq curve — end",desc:"Legacy frequency-Watt curve: over-frequency reduction reaches its limit at this frequency."}},PZ={DERFreqDroop:{label:"Frequency-Watt droop",tip:"Linearly reduces active power as frequency rises above a deadband — over-frequency curtailment (SunSpec DERFreqDroop, model 711)."},CrvSet:{label:"Frequency-Watt curve",tip:"Legacy point-based power-versus-frequency response curve (model 134)."},MustTrip:{label:"Trip thresholds",tip:"Voltage and frequency limits that disconnect the inverter from the grid when crossed (protection trips)."},DEREnterService:{label:"Enter service",tip:"The voltage/frequency window and timing the inverter must satisfy before (re)connecting after a trip."}},yZ=["DERFreqDroop","CrvSet","MustTrip","DEREnterService"],V5=new Set(["MustTrip","DEREnterService"]);function U6(Z,K){if(!Z)return K;return Z.replace(/_/g," ").replace(/\b\w/g,(Q)=>Q.toUpperCase())}function C5(Z,K){return T5[Z]?.label??U6(K??"",Z)}function D5(Z){return T5[Z]?.desc??""}var F6=[{left:"DH",right:"DI",message:"Under-frequency Watt: the low point (DH) must be below the high point (DI)."},{left:"CB",right:"CC",message:"Over-frequency Watt: the start point (CB) must be below the end point (CC)."},{left:"BN",right:"BO",message:"Enter-service voltage: the lower limit (BN) must be below the upper limit (BO)."},{left:"BP",right:"BQ",message:"Enter-service frequency: the lower limit (BP) must be below the upper limit (BQ)."},{left:"CA",right:"AF",message:"Over-frequency curtailment start (CA) must be below the over-frequency trip (AF), or the inverter trips instead of curtailing."}];function EZ(Z){let K=[];for(let Q of F6){let $=Z(Q.left),B=Z(Q.right);if($!==void 0&&B!==void 0&&!($<B))K.push(Q.message)}return K}class N5 extends q{static properties={deadband:{type:Number},slope:{type:Number},trip:{type:Number},nominal:{type:Number}};constructor(){super();this.nominal=50}static styles=U`
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
  `;render(){let Z=this.deadband,K=this.slope,Q=this.trip,$=this.nominal;if(Z===void 0||K===void 0||K<=0)return G`<div class="empty">Set the curtailment start frequency and slope to preview the curve.</div>`;let B=Z+100/K,Y=$-0.3,j=Math.max(Q??0,B,Z+1.5,$+1.5)+0.2,z=480,H=170,F=36,J=12,M=10,I=24,W=(O)=>F+(O-Y)/(j-Y)*(z-F-J),T=(O)=>M+(100-O)/100*(H-M-I),_=Math.min(B,j),qZ=Math.max(0,100-K*(_-Z)),HZ=[[Y,100],[Z,100],[_,qZ],...B<j?[[j,0]]:[]].map(([O,b5])=>`${W(O).toFixed(1)},${T(b5).toFixed(1)}`).join(" "),UZ=[];for(let O=Math.ceil(Y*2)/2;O<=j;O+=0.5)UZ.push(O);return G`
      <svg viewBox="0 0 ${z} ${H}" role="img" aria-label="Frequency-Watt curtailment curve">
        ${[0,50,100].map((O)=>R`<line class="grid" x1=${F} y1=${T(O)} x2=${z-J} y2=${T(O)} />
            <text x=${F-4} y=${T(O)+3} text-anchor="end">${O}%</text>`)}
        ${UZ.map((O)=>R`<text x=${W(O)} y=${H-I+12} text-anchor="middle">${O.toFixed(1)}</text>`)}
        <line class="frame" x1=${F} y1=${M} x2=${F} y2=${H-I} />
        <line class="frame" x1=${F} y1=${H-I} x2=${z-J} y2=${H-I} />
        <line class="dead" x1=${W(Z)} y1=${M} x2=${W(Z)} y2=${H-I} />
        <text class="lbl" x=${W(Z)} y=${M+8} text-anchor="middle">start ${Z} Hz</text>
        ${Q!==void 0&&Q>=Y&&Q<=j?R`<line class="trip" x1=${W(Q)} y1=${M} x2=${W(Q)} y2=${H-I} />
              <text x=${W(Q)} y=${H-I-4} text-anchor="middle" fill="var(--err)">trip ${Q} Hz</text>`:X}
        <polyline class="curve" points=${HZ} />
        <text x=${z/2} y=${H-2} text-anchor="middle">Power vs frequency · slope ${K} %Pref/Hz</text>
      </svg>
    `}}customElements.define("freq-watt-chart",N5);class R5 extends q{static properties={unit:{type:String},nominal:{type:Number},markers:{attribute:!1}};constructor(){super();this.unit="",this.markers=[]}static styles=U`
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
  `;render(){let Z=(this.markers??[]).filter((_)=>Number.isFinite(_.value));if(!Z.length)return G`<div class="empty">No thresholds set.</div>`;let K=Z.map((_)=>_.value).concat(this.nominal!==void 0?[this.nominal]:[]),Q=Math.min(...K),$=Math.max(...K),B=($-Q)*0.14||1;Q-=B,$+=B;let Y=480,j=70,z=10,H=10,F=34,J=(_)=>z+(_-Q)/($-Q)*(Y-z-H),M=Z.filter((_)=>_.kind==="under").map((_)=>_.value),I=Z.filter((_)=>_.kind==="over").map((_)=>_.value),W=M.length?Math.max(...M):Q,T=I.length?Math.min(...I):$;return G`
      <svg viewBox="0 0 ${Y} ${j}" role="img" aria-label="Trip thresholds">
        ${T>W?R`<rect class="band" x=${J(W)} y=${F-8} width=${J(T)-J(W)} height=16 />`:X}
        <line class="axis" x1=${z} y1=${F} x2=${Y-H} y2=${F} />
        ${this.nominal!==void 0?R`<line class="nom" x1=${J(this.nominal)} y1=${F-9} x2=${J(this.nominal)} y2=${F+9} />
              <text x=${J(this.nominal)} y=${F+20} text-anchor="middle" fill="var(--ok)">${this.nominal} ${this.unit}</text>`:X}
        ${Z.map((_,qZ)=>{let HZ=_.kind,O=qZ%2===0?F-12:F+22;return R`<line class=${HZ} x1=${J(_.value)} y1=${F-7} x2=${J(_.value)} y2=${F+7} />
            <text x=${J(_.value)} y=${O} text-anchor="middle">${_.label} ${_.value}</text>`})}
      </svg>
    `}}customElements.define("trip-line",R5);class L5 extends q{static properties={params:{attribute:!1},inverters:{attribute:!1},defaults:{attribute:!1},profile:{attribute:!1},names:{attribute:!1},busy:{attribute:!1},editing:{attribute:!1},name:{state:!0},selectedUids:{state:!0},values:{state:!0},localError:{state:!0}};constructor(){super();this.params=[],this.inverters=[],this.defaults={},this.profile=null,this.names={},this.busy=!1,this.editing=!1,this.name="",this.selectedUids=[],this.values={},this.localError=""}static styles=U`
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
  `;willUpdate(Z){if(Z.has("profile")){let K=this.profile;this.name=K?.id??"",this.selectedUids=[...K?.uids??[]];let Q={};for(let $ of K?.points??[])Q[$.aps_code]=String($.value);this.values=Q,this.localError=""}}effectiveWritable(){if(!this.selectedUids.length)return new Set;let Z=this.selectedUids.map((Q)=>new Set(this.inverters.find(($)=>$.uid===Q)?.writable_codes??[])),K=Z[0];for(let Q of Z.slice(1))K=new Set([...K].filter(($)=>Q.has($)));return K}targetDefault(Z){let K=this.defaults[Z];if(K)return{value:K.value,source:"base"};if(!this.selectedUids.length)return;let Q;for(let $ of this.selectedUids){let B=this.inverters.find((Y)=>Y.uid===$)?.current?.[Z];if(B===void 0)return;if(Q===void 0)Q=B;else if(Math.abs(B-Q)>0.000001)return}return Q===void 0?void 0:{value:Q,source:"inverter"}}effectiveValue(Z){let K=(this.values[Z]??"").trim();if(K!==""&&!Number.isNaN(Number(K)))return Number(K);return this.targetDefault(Z)?.value}isOverride(Z){let K=(this.values[Z]??"").trim();if(K===""||Number.isNaN(Number(K)))return!1;let Q=this.targetDefault(Z);return!Q||Number(K)!==Q.value}prefill(Z){if((this.values[Z]??"").trim()!=="")return;let K=this.targetDefault(Z);if(K)this.setValue(Z,String(K.value))}outOfRange(Z){let K=(this.values[Z]??"").trim();if(K===""||Number.isNaN(Number(K)))return!1;let Q=this.defaults[Z];if(!Q)return!1;let $=Number(K);return Q.min!==void 0&&$<Q.min||Q.max!==void 0&&$>Q.max}label(Z){return this.names[Z.uid]||Z.model||Z.uid}toggleTarget(Z,K){this.selectedUids=K?[...this.selectedUids,Z]:this.selectedUids.filter((Q)=>Q!==Z)}setValue(Z,K){this.values={...this.values,[Z]:K}}groups(){let Z={};for(let Q of this.params)(Z[Q.group]??=[]).push(Q);return[...yZ,...Object.keys(Z).filter((Q)=>!yZ.includes(Q))].filter((Q)=>Z[Q]?.length).map((Q)=>[Q,Z[Q]])}save=()=>{let Z=this.effectiveWritable(),K=this.params.filter(($)=>Z.has($.aps_code)&&this.isOverride($.aps_code)).map(($)=>({aps_code:$.aps_code,value:Number(this.values[$.aps_code])}));if(!this.name.trim())return void(this.localError="Profile name is required.");if(!this.selectedUids.length)return void(this.localError="Select at least one target inverter.");if(!K.length)return void(this.localError="Change at least one parameter from its default.");if(EZ(($)=>this.effectiveValue($)).length)return void(this.localError="Resolve the conflicts before saving.");this.localError="";let Q={id:this.name.trim(),uids:this.selectedUids,points:K};this.dispatchEvent(new CustomEvent("save",{detail:Q,bubbles:!0,composed:!0}))};cancel=()=>this.dispatchEvent(new CustomEvent("cancel",{bubbles:!0,composed:!0}));trips(Z){let K=[];for(let[Q,$]of Z){let B=this.effectiveValue(Q);if(B!==void 0)K.push({value:B,label:Q,kind:$})}return K}vizFor(Z){if(Z==="DERFreqDroop")return G`<freq-watt-chart
        .deadband=${this.effectiveValue("CA")}
        .slope=${this.effectiveValue("DD")}
        .trip=${this.effectiveValue("AF")}
        .nominal=${50}
      ></freq-watt-chart>`;if(Z==="CrvSet"){let K=this.trips([["DH","under"],["DI","under"],["CB","over"],["CC","over"]]);return K.length?G`<trip-line unit="Hz" .nominal=${50} .markers=${K}></trip-line>`:X}if(Z==="MustTrip"){let K=this.trips([["AC","under"],["AQ","under"],["AH","under"],["AD","over"],["AY","over"],["AB","over"],["AI","over"]]),Q=this.trips([["AE","under"],["AJ","under"],["AF","over"],["AK","over"]]);return G`
        ${K.length?G`<trip-line unit="V" .nominal=${230} .markers=${K}></trip-line>`:X}
        ${Q.length?G`<trip-line unit="Hz" .nominal=${50} .markers=${Q}></trip-line>`:X}
      `}return X}renderRow(Z,K){let Q=K.has(Z.aps_code),$=this.targetDefault(Z.aps_code),B=this.defaults[Z.aps_code],Y=(this.values[Z.aps_code]??"").trim(),j=this.isOverride(Z.aps_code),z=Q&&this.outOfRange(Z.aps_code),H=Q?this.values[Z.aps_code]??"":$?String($.value):"";return G`<tr class="${Q?"":"off"} ${j?"over":""}">
      <td>
        <div class="plabel">
          ${C5(Z.aps_code,Z.long_name)}
          ${j?G`<span class="otag">overridden</span>`:X}
          ${!Q&&$?G`<span class="rotag">read-only</span>`:X}
        </div>
        <div class="pdesc">${D5(Z.aps_code)}</div>
      </td>
      <td class="pcode">${Z.aps_code}</td>
      <td class="def">
        ${$?G`${$.value} ${Z.unit}${$.source==="inverter"?G` <span class="src" title="from the inverter's current value">inv</span>`:X}`:"—"}
      </td>
      <td class="val">
        <input
          type="number" step="any" ?disabled=${!Q}
          .value=${H}
          placeholder=${$?String($.value):Q?"—":"n/a"}
          @focus=${()=>this.prefill(Z.aps_code)}
          @input=${(F)=>this.setValue(Z.aps_code,F.target.value)}
        />
        <span class="unit">${Z.unit}</span>
        ${Q&&Y!==""?G`<button class="clear" title="Clear override" @click=${()=>this.setValue(Z.aps_code,"")}>↺</button>`:X}
        ${z?G`<span class="warn">⚠ outside base range${B?.min!==void 0?` (${B.min}–${B.max} ${Z.unit})`:""}</span>`:X}
      </td>
    </tr>`}render(){let Z=this.effectiveWritable(),K=this.selectedUids.length>0,Q=K?EZ(($)=>this.effectiveValue($)):[];return G`
      <div class="grid">
        <label class="field">
          Profile name
          <input type="text" .value=${this.name} ?disabled=${this.editing} placeholder="e.g. victron-shift"
            @input=${($)=>this.name=$.target.value} />
        </label>

        <fieldset>
          <legend>Target inverters</legend>
          <div class="targets">
            ${this.inverters.length===0?G`<span class="hint">No inverters seen yet.</span>`:this.inverters.map(($)=>G`<label class="target">
                    <input type="checkbox" .checked=${this.selectedUids.includes($.uid)}
                      @change=${(B)=>this.toggleTarget($.uid,B.target.checked)} />
                    ${this.label($)} <span class="pcode">${$.model}</span>
                  </label>`)}
          </div>
        </fieldset>

        ${!K?G`<span class="hint">Select a target to choose editable parameters.</span>`:G`
              ${Q.length?G`<div class="conflicts">⚠ Conflicting settings — resolve to save:
                    <ul>${Q.map(($)=>G`<li>${$}</li>`)}</ul>
                  </div>`:X}

              <div class="legend">
                ${this.groups().map(([$])=>{let B=PZ[$];return G`<span class="badge" title=${B?.tip??$}>${B?.label??$}</span>`})}
              </div>

              ${this.groups().map(([$,B])=>{let Y=PZ[$];return G`<details class="group" ?open=${!V5.has($)}>
                  <summary>
                    <span class="gname">${Y?.label??$}</span>
                    <span class="badge" title=${Y?.tip??$}>${$}</span>
                    <span class="gcount">${B.length} setting${B.length===1?"":"s"}</span>
                  </summary>
                  <div class="viz">${this.vizFor($)}</div>
                  <table>
                    <thead><tr><th>Setting</th><th>Code</th><th>Default</th><th>Override</th></tr></thead>
                    <tbody>${B.map((j)=>this.renderRow(j,Z))}</tbody>
                  </table>
                </details>`})}

              ${this.selectedUids.length>1?G`<div class="hint">Greyed rows are not writable on every selected target.</div>`:X}
            `}

        ${this.localError?G`<div class="err">⚠ ${this.localError}</div>`:X}

        <div class="actions">
          <button class="save" @click=${this.save} ?disabled=${this.busy||Q.length>0}>
            ${this.busy?"Applying…":"Save & apply"}
          </button>
          <button class="cancel" @click=${this.cancel} ?disabled=${this.busy}>Cancel</button>
          <span class="hint">${Q.length?"resolve conflicts to save":"applies to the selected inverters"}</span>
        </div>
      </div>
    `}}customElements.define("local-site-profile-form",L5);class S5 extends q{static properties={data:{state:!0},names:{state:!0},error:{state:!0},notice:{state:!0},baseBusy:{state:!0},overlayBusy:{state:!0},editing:{state:!0},editingExisting:{state:!0}};constructor(){super();this.data=null,this.names={},this.error="",this.notice="",this.baseBusy=!1,this.overlayBusy=!1,this.editing=null,this.editingExisting=!1}static styles=U`
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
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){try{let[Z,K]=await Promise.all([A.profiles(),A.getSettings()]);this.data=Z,this.error=Z.error??"",this.names=K.settings?.inverter_names??{}}catch(Z){this.error=Z.message}}invName(Z){if(this.names[Z])return this.names[Z];return this.data?.inverters.find((Q)=>Q.uid===Z)?.model||Z}onSelectBase=async(Z)=>{let K=Z.detail;if(!window.confirm(`Apply base grid profile "${K}" to every inverter? This writes grid-protection settings across the whole fleet.`))return;this.baseBusy=!0,this.notice="",this.error="";try{await A.selectBase(K),await this.load(),this.notice=`Base profile "${K}" applied.`}catch(Q){this.error=Q.message}finally{this.baseBusy=!1}};newProfile(){this.editing={id:"",uids:[],points:[]},this.editingExisting=!1,this.notice="",this.error=""}editProfile(Z){this.editing=Z,this.editingExisting=!0,this.notice="",this.error=""}onCancelEdit=()=>{this.editing=null};exportProfile(Z){let K={id:Z.id,uids:Z.uids,points:Z.points.map((Y)=>({aps_code:Y.aps_code,value:Y.value}))},Q=new Blob([JSON.stringify(K,null,2)],{type:"application/json"}),$=URL.createObjectURL(Q),B=document.createElement("a");B.href=$,B.download=`${Z.id||"profile"}.json`,B.click(),URL.revokeObjectURL($)}triggerImport=()=>{this.shadowRoot?.querySelector("#importfile")?.click()};onImportFile=async(Z)=>{let K=Z.target,Q=K.files?.[0];if(K.value="",!Q)return;try{let $=JSON.parse(await Q.text());if(!$||!Array.isArray($.points))throw Error("not a profile (no points)");let B={id:typeof $.id==="string"?$.id:"",uids:Array.isArray($.uids)?$.uids.filter((Y)=>typeof Y==="string"):[],points:$.points.filter((Y)=>typeof Y?.aps_code==="string"&&typeof Y?.value==="number").map((Y)=>({aps_code:Y.aps_code,value:Y.value}))};this.editing=B,this.editingExisting=!1,this.error="",this.notice=`Imported "${B.id||"profile"}" — review the targets and values, then Save.`}catch($){this.error="Import failed: "+$.message}};onSaveOverlay=async(Z)=>{let K=Z.detail;if(!window.confirm(`Apply Local Site profile "${K.id}" to ${K.uids.length} inverter(s)? This writes grid-protection parameters to each.`))return;this.overlayBusy=!0,this.notice="",this.error="";try{let Q=await A.saveOverlay(K);this.editing=null,await this.load(),this.reportResults(K.id,Q.results)}catch(Q){this.error=Q.message}finally{this.overlayBusy=!1}};deleteProfile=async(Z)=>{if(!window.confirm(`Delete Local Site profile "${Z.id}" and clear it from ${Z.uids.length} inverter(s)?`))return;this.overlayBusy=!0,this.notice="",this.error="";try{let K=await A.deleteOverlay(Z.id,Z.uids);if(this.editing?.id===Z.id)this.editing=null;await this.load(),this.reportResults(Z.id,K.results,"cleared")}catch(K){this.error=K.message}finally{this.overlayBusy=!1}};reportResults(Z,K,Q="applied"){let $=K.filter((B)=>!B.ok);if($.length===0)this.notice=`Profile "${Z}" ${Q} to ${K.length} inverter(s).`;else{let B=Q==="cleared"?"clearing":"applying",Y=$.map((j)=>`${this.invName(j.uid)}: ${j.error||"unconfirmed"}`).join("; ");this.notice=`Profile "${Z}" saved on the ECU, but ${B} was not confirmed on ${$.length} of ${K.length} inverter(s) (offline?) — ${Y}`}}renderBase(){let Z=this.data?.base;return G`
      <div class="panel">
        <h2>Base grid profile</h2>
        <grid-profile-form
          .profiles=${Z?.profiles??[]}
          .activeBase=${Z?.active_base??""}
          .reconcilerReady=${Z?.reconciler_ready??!1}
          .busy=${this.baseBusy}
          @apply=${this.onSelectBase}
        ></grid-profile-form>
      </div>
    `}renderLocalSite(){let Z=this.data;return G`
      <div class="panel">
        <div class="row">
          <h2 style="margin:0">Local Site profiles</h2>
          ${this.editing===null?G`<div class="hdr-actions">
                <button class="ghost" @click=${this.triggerImport}>Import</button>
                <button class="primary" @click=${()=>this.newProfile()}>+ New profile</button>
              </div>`:X}
        </div>
        <input id="importfile" type="file" accept=".json,application/json" hidden @change=${this.onImportFile} />

        ${this.editing!==null?G`<local-site-profile-form
              .params=${Z?.params??[]}
              .inverters=${Z?.inverters??[]}
              .defaults=${Z?.base_defaults??{}}
              .names=${this.names}
              .profile=${this.editing}
              .editing=${this.editingExisting}
              .busy=${this.overlayBusy}
              @save=${this.onSaveOverlay}
              @cancel=${this.onCancelEdit}
            ></local-site-profile-form>`:this.renderCards()}
      </div>
    `}renderCards(){let Z=this.data?.overlays??[];if(Z.length===0)return G`<div class="empty">No Local Site profiles yet. Create one to override grid-protection parameters on specific inverters.</div>`;return G`<div class="cards">
      ${Z.map((K)=>G`<div class="card">
          <div class="title">${K.id}</div>
          <div class="meta">Targets: ${K.uids.map((Q)=>this.invName(Q)).join(", ")||"none"}</div>
          <div class="chips">
            ${K.points.map((Q)=>G`<span class="chip">${Q.aps_code} = ${Q.value}${Q.unit?` ${Q.unit}`:""}</span>`)}
          </div>
          <div class="cardactions">
            <button @click=${()=>this.editProfile(K)}>Edit</button>
            <button @click=${()=>this.exportProfile(K)}>Export</button>
            <button class="del" @click=${()=>this.deleteProfile(K)}>Delete</button>
          </div>
        </div>`)}
    </div>`}render(){return G`
      ${this.notice?G`<div class="banner ok">${this.notice}</div>`:X}
      ${this.error?G`<div class="banner err">⚠ ${this.error}</div>`:X}
      ${this.data===null?G`<div class="panel"><div class="loading">Loading…</div></div>`:G`<div class="cols">
            <div>${this.renderLocalSite()}</div>
            <div>${this.renderBase()}</div>
          </div>`}
    `}}customElements.define("profiles-view",S5);class P5 extends q{static properties={settings:{attribute:!1}};constructor(){super();this.settings={ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}static styles=U`
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
  `;save=()=>{let Z=this.shadowRoot;if(!Z)return;let K=($)=>(Z.querySelector(`#${$}`)?.value??"").trim(),Q={ecu_id:K("ecu_id"),mac:K("mac"),pan_override:K("pan_override"),zigbee_type:K("zigbee_type")};this.dispatchEvent(new CustomEvent("save",{detail:Q,bubbles:!0,composed:!0}))};render(){let Z=this.settings;return G`
      <div class="grid">
        <label>
          ECU ID
          <input id="ecu_id" type="text" .value=${Z.ecu_id??""} />
        </label>
        <label>
          MAC
          <input id="mac" type="text" .value=${Z.mac??""} />
        </label>
        <label>
          PAN override
          <input id="pan_override" type="text" placeholder="auto from MAC" .value=${Z.pan_override??""} />
        </label>
        <label>
          ZigBee type
          <select id="zigbee_type" .value=${Z.zigbee_type||"apsystems"}>
            <option value="apsystems">apsystems</option>
            <option value="general">general</option>
          </select>
        </label>
        <div class="actions">
          <button class="save" @click=${this.save}>Save</button>
        </div>
      </div>
    `}}customElements.define("settings-form",P5);class y5 extends q{static properties={settings:{state:!0},error:{state:!0},notice:{state:!0},loading:{state:!0},saving:{state:!0}};constructor(){super();this.settings=null,this.error="",this.notice="",this.loading=!1,this.saving=!1}static styles=U`
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
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){this.loading=!0;try{let Z=await A.getSettings();this.settings=Z.settings??null,this.error=Z.error??""}catch(Z){this.error=Z.message}finally{this.loading=!1}}onSave=async(Z)=>{this.saving=!0,this.notice="",this.error="";try{this.settings=await A.saveSettings(Z.detail),this.notice="Settings saved."}catch(K){this.error=K.message}finally{this.saving=!1,await this.load()}};render(){return G`
      <div class="panel">
        <h2>ECU settings</h2>
        ${this.notice?G`<div class="banner ok">${this.notice}</div>`:X}
        ${this.error?G`<div class="banner err">⚠ ${this.error}</div>`:X}
        ${this.loading&&!this.settings?G`<div class="loading">Loading…</div>`:G`<settings-form
              .settings=${this.settings??{ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}
              @save=${this.onSave}
            ></settings-form>`}
      </div>
    `}}customElements.define("settings-view",y5);class E5 extends q{static properties={items:{attribute:!1},route:{type:String},open:{type:Boolean}};constructor(){super();this.items=[],this.route="dashboard",this.open=!1}close=()=>{this.dispatchEvent(new CustomEvent("close",{bubbles:!0,composed:!0}))};static styles=U`
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
  `;render(){return G`
      <nav class=${this.open?"open":""}>
        <div class="brand">ECU CONSOLE</div>
        ${this.items.map((Z)=>G`<a
            class="item ${this.route===Z.id?"active":""}"
            href="#/${Z.id}"
            @click=${this.close}
          ><span class="ic">${Z.icon}</span>${Z.label}</a>`)}
      </nav>
      ${this.open?G`<div class="scrim" @click=${this.close}></div>`:X}
    `}}customElements.define("app-nav",E5);var xZ=[{id:"dashboard",label:"Dashboard",icon:"▮▮"},{id:"inverters",label:"Inverters",icon:"⌁"},{id:"alarms",label:"Alarms",icon:"!"},{id:"events",label:"Events",icon:"≣"},{id:"profiles",label:"Profiles",icon:"⛭"},{id:"settings",label:"Settings",icon:"⚙"}];class x5 extends q{static properties={ready:{state:!0},authed:{state:!0},configured:{state:!0},route:{state:!0},fleet:{state:!0},system:{state:!0},names:{state:!0},customProfiles:{state:!0},navOpen:{state:!0}};closeSSE=null;sysTimer=null;settingsCache=null;constructor(){super();this.ready=!1,this.authed=!1,this.configured=!0,this.route="dashboard",this.fleet=null,this.system=null,this.names={},this.customProfiles={},this.navOpen=!1}static styles=U`
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
  `;connectedCallback(){super.connectedCallback(),window.addEventListener("hashchange",this.onHash),this.onHash(),this.init()}disconnectedCallback(){super.disconnectedCallback(),window.removeEventListener("hashchange",this.onHash),this.stopStreams()}onHash=()=>{let Z=(location.hash.replace(/^#\/?/,"")||"dashboard").split("/")[0];if(this.route=xZ.some((K)=>K.id===Z)?Z:"dashboard",this.navOpen=!1,this.route==="dashboard"&&this.authed)this.fetchOverlays()};async init(){try{let Z=await A.authStatus();if(this.configured=Z.configured,this.authed=Z.authenticated,this.authed)this.startStreams()}catch{}finally{this.ready=!0}}onAuthed=async()=>{this.authed=!0,this.startStreams()};logout=async()=>{try{await A.logout()}catch{}this.authed=!1,this.stopStreams(),this.fleet=null,this.system=null};startStreams(){this.stopStreams(),this.closeSSE=Y5((K)=>{this.fleet=K});let Z=()=>A.system().then((K)=>this.system=K).catch(()=>{});Z(),this.sysTimer=setInterval(Z,5000),this.fetchSettings(),this.fetchOverlays()}async fetchSettings(){try{let Z=await A.getSettings();if(Z.settings)this.settingsCache=Z.settings,this.names=Z.settings.inverter_names??{}}catch{}}async fetchOverlays(){try{let Z=await A.overlays(),K={};for(let Q of Z)for(let $ of Q.uids)K[$]=Q.id;this.customProfiles=K}catch{}}onRename=async(Z)=>{let{uid:K,name:Q}=Z.detail,$=this.settingsCache??{ecu_id:"",mac:"",pan_override:"",zigbee_type:""},B={...$.inverter_names??{}};if(Q.trim())B[K]=Q.trim();else delete B[K];let Y={...$,inverter_names:B};try{await A.saveSettings(Y),this.settingsCache=Y,this.names=B}catch{}};stopStreams(){if(this.closeSSE?.(),this.closeSSE=null,this.sysTimer)clearInterval(this.sysTimer);this.sysTimer=null}activeView(){switch(this.route){case"inverters":return G`<inverters-view
          .fleet=${this.fleet}
          .names=${this.names}
          @rename=${this.onRename}
        ></inverters-view>`;case"alarms":return G`<alarms-view .fleet=${this.fleet}></alarms-view>`;case"events":return G`<events-view></events-view>`;case"profiles":return G`<profiles-view></profiles-view>`;case"settings":return G`<settings-view></settings-view>`;default:return G`<dashboard-view
          .fleet=${this.fleet}
          .system=${this.system}
          .names=${this.names}
          .profiles=${this.customProfiles}
        ></dashboard-view>`}}render(){if(!this.ready)return X;if(!this.authed)return G`<login-view .configured=${this.configured} @authed=${this.onAuthed}></login-view>`;let Z=xZ.find((Q)=>Q.id===this.route)?.label??"Dashboard",K=this.system?.invdriver_connected??!1;return G`
      <div class="layout">
        <app-nav
          .items=${xZ}
          .route=${this.route}
          .open=${this.navOpen}
          @close=${()=>this.navOpen=!1}
        ></app-nav>
        <main>
          <div class="topbar">
            <div class="titlewrap">
              <button class="hamburger" aria-label="Menu" aria-expanded=${this.navOpen} @click=${()=>this.navOpen=!this.navOpen}>☰</button>
              <h1>${Z}</h1>
            </div>
            <div class="right">
              <span class="conn">
                <span class="dot ${K?"on":"off"}"></span>
                inv-driver ${K?"connected":"down"}
              </span>
              <button class="logout" @click=${this.logout}>Sign out</button>
            </div>
          </div>
          ${this.activeView()}
        </main>
      </div>
    `}}customElements.define("ecu-app",x5);
