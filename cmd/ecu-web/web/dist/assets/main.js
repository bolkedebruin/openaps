var ZZ=globalThis,kZ=ZZ.ShadowRoot&&(ZZ.ShadyCSS===void 0||ZZ.ShadyCSS.nativeShadow)&&"adoptedStyleSheets"in Document.prototype&&"replace"in CSSStyleSheet.prototype,FZ=Symbol(),bZ=new WeakMap;class WZ{constructor(Z,K,Q){if(this._$cssResult$=!0,Q!==FZ)throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");this.cssText=Z,this._strings=K}get styleSheet(){let Z=this._styleSheet,K=this._strings;if(kZ&&Z===void 0){let Q=K!==void 0&&K.length===1;if(Q)Z=bZ.get(K);if(Z===void 0){if((this._styleSheet=Z=new CSSStyleSheet).replaceSync(this.cssText),Q)bZ.set(K,Z)}}return Z}toString(){return this.cssText}}var f5=(Z)=>{if(Z._$cssResult$===!0)return Z.cssText;else if(typeof Z==="number")return Z;else throw Error(`Value passed to 'css' function must be a 'css' function result: ${Z}. Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.`)},h5=(Z)=>new WZ(typeof Z==="string"?Z:String(Z),void 0,FZ),H=(Z,...K)=>{let Q=Z.length===1?Z[0]:K.reduce(($,B,Y)=>$+f5(B)+Z[Y+1],Z[0]);return new WZ(Q,Z,FZ)},wZ=(Z,K)=>{if(kZ)Z.adoptedStyleSheets=K.map((Q)=>Q instanceof CSSStyleSheet?Q:Q.styleSheet);else for(let Q of K){let $=document.createElement("style"),B=ZZ.litNonce;if(B!==void 0)$.setAttribute("nonce",B);$.textContent=Q.cssText,Z.appendChild($)}},g5=(Z)=>{let K="";for(let Q of Z.cssRules)K+=Q.cssText;return h5(K)},_Z=kZ?(Z)=>Z:(Z)=>Z instanceof CSSStyleSheet?g5(Z):Z;var{is:c5,defineProperty:d5,getOwnPropertyDescriptor:fZ,getOwnPropertyNames:u5,getOwnPropertySymbols:v5,getPrototypeOf:hZ}=Object,m5=!1,V=globalThis;if(m5)V.customElements??=customElements;var D=!0,P,gZ=V.trustedTypes,p5=gZ?gZ.emptyScript:"",dZ=D?V.reactiveElementPolyfillSupportDevMode:V.reactiveElementPolyfillSupport;if(D)V.litIssuedWarnings??=new Set,P=(Z,K)=>{if(K+=` See https://lit.dev/msg/${Z} for more information.`,!V.litIssuedWarnings.has(K)&&!V.litIssuedWarnings.has(Z))console.warn(K),V.litIssuedWarnings.add(K)},queueMicrotask(()=>{if(P("dev-mode","Lit is in dev mode. Not recommended for production!"),V.ShadyDOM?.inUse&&dZ===void 0)P("polyfill-support-missing","Shadow DOM is being polyfilled via `ShadyDOM` but the `polyfill-support` module has not been loaded.")});var o5=D?(Z)=>{if(!V.emitLitDebugLogEvents)return;V.dispatchEvent(new CustomEvent("lit-debug",{detail:Z}))}:void 0,d=(Z,K)=>Z,AZ={toAttribute(Z,K){switch(K){case Boolean:Z=Z?p5:null;break;case Object:case Array:Z=Z==null?Z:JSON.stringify(Z);break}return Z},fromAttribute(Z,K){let Q=Z;switch(K){case Boolean:Q=Z!==null;break;case Number:Q=Z===null?null:Number(Z);break;case Object:case Array:try{Q=JSON.parse(Z)}catch($){Q=null}break}return Q}},uZ=(Z,K)=>!c5(Z,K),cZ={attribute:!0,type:String,converter:AZ,reflect:!1,useDefault:!1,hasChanged:uZ};Symbol.metadata??=Symbol("metadata");V.litPropertyMetadata??=new WeakMap;class R extends HTMLElement{static addInitializer(Z){this.__prepare(),(this._initializers??=[]).push(Z)}static get observedAttributes(){return this.finalize(),this.__attributeToPropertyMap&&[...this.__attributeToPropertyMap.keys()]}static createProperty(Z,K=cZ){if(K.state)K.attribute=!1;if(this.__prepare(),this.prototype.hasOwnProperty(Z))K=Object.create(K),K.wrapped=!0;if(this.elementProperties.set(Z,K),!K.noAccessor){let Q=D?Symbol.for(`${String(Z)} (@property() cache)`):Symbol(),$=this.getPropertyDescriptor(Z,Q,K);if($!==void 0)d5(this.prototype,Z,$)}}static getPropertyDescriptor(Z,K,Q){let{get:$,set:B}=fZ(this.prototype,Z)??{get(){return this[K]},set(Y){this[K]=Y}};if(D&&$==null){if("value"in(fZ(this.prototype,Z)??{}))throw Error(`Field ${JSON.stringify(String(Z))} on ${this.name} was declared as a reactive property but it's actually declared as a value on the prototype. Usually this is due to using @property or @state on a method.`);P("reactive-property-without-getter",`Field ${JSON.stringify(String(Z))} on ${this.name} was declared as a reactive property but it does not have a getter. This will be an error in a future version of Lit.`)}return{get:$,set(Y){let X=$?.call(this);B?.call(this,Y),this.requestUpdate(Z,X,Q)},configurable:!0,enumerable:!0}}static getPropertyOptions(Z){return this.elementProperties.get(Z)??cZ}static __prepare(){if(this.hasOwnProperty(d("elementProperties",this)))return;let Z=hZ(this);if(Z.finalize(),Z._initializers!==void 0)this._initializers=[...Z._initializers];this.elementProperties=new Map(Z.elementProperties)}static finalize(){if(this.hasOwnProperty(d("finalized",this)))return;if(this.finalized=!0,this.__prepare(),this.hasOwnProperty(d("properties",this))){let K=this.properties,Q=[...u5(K),...v5(K)];for(let $ of Q)this.createProperty($,K[$])}let Z=this[Symbol.metadata];if(Z!==null){let K=litPropertyMetadata.get(Z);if(K!==void 0)for(let[Q,$]of K)this.elementProperties.set(Q,$)}this.__attributeToPropertyMap=new Map;for(let[K,Q]of this.elementProperties){let $=this.__attributeNameForProperty(K,Q);if($!==void 0)this.__attributeToPropertyMap.set($,K)}if(this.elementStyles=this.finalizeStyles(this.styles),D){if(this.hasOwnProperty("createProperty"))P("no-override-create-property","Overriding ReactiveElement.createProperty() is deprecated. The override will not be called with standard decorators");if(this.hasOwnProperty("getPropertyDescriptor"))P("no-override-get-property-descriptor","Overriding ReactiveElement.getPropertyDescriptor() is deprecated. The override will not be called with standard decorators")}}static finalizeStyles(Z){let K=[];if(Array.isArray(Z)){let Q=new Set(Z.flat(1/0).reverse());for(let $ of Q)K.unshift(_Z($))}else if(Z!==void 0)K.push(_Z(Z));return K}static __attributeNameForProperty(Z,K){let Q=K.attribute;return Q===!1?void 0:typeof Q==="string"?Q:typeof Z==="string"?Z.toLowerCase():void 0}constructor(){super();this.__instanceProperties=void 0,this.isUpdatePending=!1,this.hasUpdated=!1,this.__reflectingProperty=null,this.__initialize()}__initialize(){this.__updatePromise=new Promise((Z)=>this.enableUpdating=Z),this._$changedProperties=new Map,this.__saveInstanceProperties(),this.requestUpdate(),this.constructor._initializers?.forEach((Z)=>Z(this))}addController(Z){if((this.__controllers??=new Set).add(Z),this.renderRoot!==void 0&&this.isConnected)Z.hostConnected?.()}removeController(Z){this.__controllers?.delete(Z)}__saveInstanceProperties(){let Z=new Map,K=this.constructor.elementProperties;for(let Q of K.keys())if(this.hasOwnProperty(Q))Z.set(Q,this[Q]),delete this[Q];if(Z.size>0)this.__instanceProperties=Z}createRenderRoot(){let Z=this.shadowRoot??this.attachShadow(this.constructor.shadowRootOptions);return wZ(Z,this.constructor.elementStyles),Z}connectedCallback(){this.renderRoot??=this.createRenderRoot(),this.enableUpdating(!0),this.__controllers?.forEach((Z)=>Z.hostConnected?.())}enableUpdating(Z){}disconnectedCallback(){this.__controllers?.forEach((Z)=>Z.hostDisconnected?.())}attributeChangedCallback(Z,K,Q){this._$attributeToProperty(Z,Q)}__propertyToAttribute(Z,K){let $=this.constructor.elementProperties.get(Z),B=this.constructor.__attributeNameForProperty(Z,$);if(B!==void 0&&$.reflect===!0){let X=($.converter?.toAttribute!==void 0?$.converter:AZ).toAttribute(K,$.type);if(D&&this.constructor.enabledWarnings.includes("migration")&&X===void 0)P("undefined-attribute-value",`The attribute value for the ${Z} property is undefined on element ${this.localName}. The attribute will be removed, but in the previous version of \`ReactiveElement\`, the attribute would not have changed.`);if(this.__reflectingProperty=Z,X==null)this.removeAttribute(B);else this.setAttribute(B,X);this.__reflectingProperty=null}}_$attributeToProperty(Z,K){let Q=this.constructor,$=Q.__attributeToPropertyMap.get(Z);if($!==void 0&&this.__reflectingProperty!==$){let B=Q.getPropertyOptions($),Y=typeof B.converter==="function"?{fromAttribute:B.converter}:B.converter?.fromAttribute!==void 0?B.converter:AZ;this.__reflectingProperty=$;let X=Y.fromAttribute(K,B.type);this[$]=X??this.__defaultValues?.get($)??X,this.__reflectingProperty=null}}requestUpdate(Z,K,Q,$=!1,B){if(Z!==void 0){if(D&&Z instanceof Event)P("","The requestUpdate() method was called with an Event as the property name. This is probably a mistake caused by binding this.requestUpdate as an event listener. Instead bind a function that will call it with no arguments: () => this.requestUpdate()");let Y=this.constructor;if($===!1)B=this[Z];if(Q??=Y.getPropertyOptions(Z),(Q.hasChanged??uZ)(B,K)||Q.useDefault&&Q.reflect&&B===this.__defaultValues?.get(Z)&&!this.hasAttribute(Y.__attributeNameForProperty(Z,Q)))this._$changeProperty(Z,K,Q);else return}if(this.isUpdatePending===!1)this.__updatePromise=this.__enqueueUpdate()}_$changeProperty(Z,K,{useDefault:Q,reflect:$,wrapped:B},Y){if(Q&&!(this.__defaultValues??=new Map).has(Z)){if(this.__defaultValues.set(Z,Y??K??this[Z]),B!==!0||Y!==void 0)return}if(!this._$changedProperties.has(Z)){if(!this.hasUpdated&&!Q)K=void 0;this._$changedProperties.set(Z,K)}if($===!0&&this.__reflectingProperty!==Z)(this.__reflectingProperties??=new Set).add(Z)}async __enqueueUpdate(){this.isUpdatePending=!0;try{await this.__updatePromise}catch(K){Promise.reject(K)}let Z=this.scheduleUpdate();if(Z!=null)await Z;return!this.isUpdatePending}scheduleUpdate(){let Z=this.performUpdate();if(D&&this.constructor.enabledWarnings.includes("async-perform-update")&&typeof Z?.then==="function")P("async-perform-update",`Element ${this.localName} returned a Promise from performUpdate(). This behavior is deprecated and will be removed in a future version of ReactiveElement.`);return Z}performUpdate(){if(!this.isUpdatePending)return;if(o5?.({kind:"update"}),!this.hasUpdated){if(this.renderRoot??=this.createRenderRoot(),D){let B=[...this.constructor.elementProperties.keys()].filter((Y)=>this.hasOwnProperty(Y)&&(Y in hZ(this)));if(B.length)throw Error(`The following properties on element ${this.localName} will not trigger updates as expected because they are set using class fields: ${B.join(", ")}. Native class fields and some compiled output will overwrite accessors used for detecting changes. See https://lit.dev/msg/class-field-shadowing for more information.`)}if(this.__instanceProperties){for(let[$,B]of this.__instanceProperties)this[$]=B;this.__instanceProperties=void 0}let Q=this.constructor.elementProperties;if(Q.size>0)for(let[$,B]of Q){let{wrapped:Y}=B,X=this[$];if(Y===!0&&!this._$changedProperties.has($)&&X!==void 0)this._$changeProperty($,void 0,B,X)}}let Z=!1,K=this._$changedProperties;try{if(Z=this.shouldUpdate(K),Z)this.willUpdate(K),this.__controllers?.forEach((Q)=>Q.hostUpdate?.()),this.update(K);else this.__markUpdated()}catch(Q){throw Z=!1,this.__markUpdated(),Q}if(Z)this._$didUpdate(K)}willUpdate(Z){}_$didUpdate(Z){if(this.__controllers?.forEach((K)=>K.hostUpdated?.()),!this.hasUpdated)this.hasUpdated=!0,this.firstUpdated(Z);if(this.updated(Z),D&&this.isUpdatePending&&this.constructor.enabledWarnings.includes("change-in-update"))P("change-in-update",`Element ${this.localName} scheduled an update (generally because a property was set) after an update completed, causing a new update to be scheduled. This is inefficient and should be avoided unless the next update can only be scheduled as a side effect of the previous update.`)}__markUpdated(){this._$changedProperties=new Map,this.isUpdatePending=!1}get updateComplete(){return this.getUpdateComplete()}getUpdateComplete(){return this.__updatePromise}shouldUpdate(Z){return!0}update(Z){this.__reflectingProperties&&=this.__reflectingProperties.forEach((K)=>this.__propertyToAttribute(K,this[K])),this.__markUpdated()}updated(Z){}firstUpdated(Z){}}R.elementStyles=[];R.shadowRootOptions={mode:"open"};R[d("elementProperties",R)]=new Map;R[d("finalized",R)]=new Map;dZ?.({ReactiveElement:R});if(D){R.enabledWarnings=["change-in-update","async-perform-update"];let Z=function(K){if(!K.hasOwnProperty(d("enabledWarnings",K)))K.enabledWarnings=K.enabledWarnings.slice()};R.enableWarning=function(K){if(Z(this),!this.enabledWarnings.includes(K))this.enabledWarnings.push(K)},R.disableWarning=function(K){Z(this);let Q=this.enabledWarnings.indexOf(K);if(Q>=0)this.enabledWarnings.splice(Q,1)}}(V.reactiveElementVersions??=[]).push("2.1.2");if(D&&V.reactiveElementVersions.length>1)queueMicrotask(()=>{P("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var N=globalThis,W=(Z)=>{if(!N.emitLitDebugLogEvents)return;N.dispatchEvent(new CustomEvent("lit-debug",{detail:Z}))},l5=0,o;N.litIssuedWarnings??=new Set,o=(Z,K)=>{if(K+=Z?` See https://lit.dev/msg/${Z} for more information.`:"",!N.litIssuedWarnings.has(K)&&!N.litIssuedWarnings.has(Z))console.warn(K),N.litIssuedWarnings.add(K)},queueMicrotask(()=>{o("dev-mode","Lit is in dev mode. Not recommended for production!")});var y=N.ShadyDOM?.inUse&&N.ShadyDOM?.noPatch===!0?N.ShadyDOM.wrap:(Z)=>Z,KZ=N.trustedTypes,vZ=KZ?KZ.createPolicy("lit-html",{createHTML:(Z)=>Z}):void 0,s5=(Z)=>Z,GZ=(Z,K,Q)=>s5,i5=(Z)=>{if(c!==GZ)throw Error("Attempted to overwrite existing lit-html security policy. setSanitizeDOMValueFactory should be called at most once.");c=Z},r5=()=>{c=GZ},VZ=(Z,K,Q)=>{return c(Z,K,Q)},rZ="$lit$",E=`lit$${Math.random().toFixed(9).slice(2)}$`,aZ="?"+E,a5=`<${aZ}>`,h=document,l=()=>h.createComment(""),s=(Z)=>Z===null||typeof Z!="object"&&typeof Z!="function",DZ=Array.isArray,n5=(Z)=>DZ(Z)||typeof Z?.[Symbol.iterator]==="function",OZ=`[ 	
\f\r]`,t5=`[^ 	
\f\r"'\`<>=]`,e5=`[^\\s"'>=/]`,m=/<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g,mZ=1,IZ=2,Z6=3,pZ=/-->/g,oZ=/>/g,b=new RegExp(`>|${OZ}(?:(${e5}+)(${OZ}*=${OZ}*(?:${t5}|("|')|))|$)`,"g"),K6=0,lZ=1,Q6=2,sZ=3,TZ=/'/g,CZ=/"/g,nZ=/^(?:script|style|textarea|title)$/i,$6=1,QZ=2,$Z=3,RZ=1,BZ=2,B6=3,G6=4,Y6=5,NZ=6,j6=7,LZ=(Z)=>(K,...Q)=>{if(K.some(($)=>$===void 0))console.warn(`Some template strings are undefined.
This is probably caused by illegal octal escape sequences.`);if(Q.some(($)=>$?._$litStatic$))o("",`Static values 'literal' or 'unsafeStatic' cannot be used as values to non-static templates.
Please use the static 'html' tag function. See https://lit.dev/docs/templates/expressions/#static-expressions`);return{["_$litType$"]:Z,strings:K,values:Q}},G=LZ($6),L=LZ(QZ),A6=LZ($Z),g=Symbol.for("lit-noChange"),j=Symbol.for("lit-nothing"),iZ=new WeakMap,f=h.createTreeWalker(h,129),c=GZ;function tZ(Z,K){if(!DZ(Z)||!Z.hasOwnProperty("raw")){let Q="invalid template strings array";throw Q=`
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
`),Error(Q)}return vZ!==void 0?vZ.createHTML(K):K}var X6=(Z,K)=>{let Q=Z.length-1,$=[],B=K===QZ?"<svg>":K===$Z?"<math>":"",Y,X=m;for(let q=0;q<Q;q++){let M=Z[q],z=-1,k,_=0,F;while(_<M.length){if(X.lastIndex=_,F=X.exec(M),F===null)break;if(_=X.lastIndex,X===m){if(F[mZ]==="!--")X=pZ;else if(F[mZ]!==void 0)X=oZ;else if(F[IZ]!==void 0){if(nZ.test(F[IZ]))Y=new RegExp(`</${F[IZ]}`,"g");X=b}else if(F[Z6]!==void 0)throw Error("Bindings in tag names are not supported. Please use static templates instead. See https://lit.dev/docs/templates/expressions/#static-expressions")}else if(X===b)if(F[K6]===">")X=Y??m,z=-1;else if(F[lZ]===void 0)z=-2;else z=X.lastIndex-F[Q6].length,k=F[lZ],X=F[sZ]===void 0?b:F[sZ]==='"'?CZ:TZ;else if(X===CZ||X===TZ)X=b;else if(X===pZ||X===oZ)X=m;else X=b,Y=void 0}console.assert(z===-1||X===b||X===TZ||X===CZ,"unexpected parse state B");let C=X===b&&Z[q+1].startsWith("/>")?" ":"";B+=X===m?M+a5:z>=0?($.push(k),M.slice(0,z)+rZ+M.slice(z))+E+C:M+E+(z===-2?q:C)}let J=B+(Z[Q]||"<?>")+(K===QZ?"</svg>":K===$Z?"</math>":"");return[tZ(Z,J),$]};class i{constructor({strings:Z,["_$litType$"]:K},Q){this.parts=[];let $,B=0,Y=0,X=Z.length-1,J=this.parts,[q,M]=X6(Z,K);if(this.el=i.createElement(q,Q),f.currentNode=this.el.content,K===QZ||K===$Z){let z=this.el.content.firstChild;z.replaceWith(...z.childNodes)}while(($=f.nextNode())!==null&&J.length<X){if($.nodeType===1){{let z=$.localName;if(/^(?:textarea|template)$/i.test(z)&&$.innerHTML.includes(E)){let k=`Expressions are not supported inside \`${z}\` elements. See https://lit.dev/msg/expression-in-${z} for more information.`;if(z==="template")throw Error(k);else o("",k)}}if($.hasAttributes()){for(let z of $.getAttributeNames())if(z.endsWith(rZ)){let k=M[Y++],F=$.getAttribute(z).split(E),C=/([.?@])?(.*)/.exec(k);J.push({type:RZ,index:B,name:C[2],strings:F,ctor:C[1]==="."?Z5:C[1]==="?"?K5:C[1]==="@"?Q5:a}),$.removeAttribute(z)}else if(z.startsWith(E))J.push({type:NZ,index:B}),$.removeAttribute(z)}if(nZ.test($.tagName)){let z=$.textContent.split(E),k=z.length-1;if(k>0){$.textContent=KZ?KZ.emptyScript:"";for(let _=0;_<k;_++)$.append(z[_],l()),f.nextNode(),J.push({type:BZ,index:++B});$.append(z[k],l())}}}else if($.nodeType===8)if($.data===aZ)J.push({type:BZ,index:B});else{let k=-1;while((k=$.data.indexOf(E,k+1))!==-1)J.push({type:j6,index:B}),k+=E.length-1}B++}if(M.length!==Y)throw Error('Detected duplicate attribute bindings. This occurs if your template has duplicate attributes on an element tag. For example "<input ?disabled=${true} ?disabled=${false}>" contains a duplicate "disabled" attribute. The error was detected in the following template: \n`'+Z.join("${...}")+"`");W&&W({kind:"template prep",template:this,clonableTemplate:this.el,parts:this.parts,strings:Z})}static createElement(Z,K){let Q=h.createElement("template");return Q.innerHTML=Z,Q}}function u(Z,K,Q=Z,$){if(K===g)return K;let B=$!==void 0?Q.__directives?.[$]:Q.__directive,Y=s(K)?void 0:K._$litDirective$;if(B?.constructor!==Y){if(B?._$notifyDirectiveConnectionChanged?.(!1),Y===void 0)B=void 0;else B=new Y(Z),B._$initialize(Z,Q,$);if($!==void 0)(Q.__directives??=[])[$]=B;else Q.__directive=B}if(B!==void 0)K=u(Z,B._$resolve(Z,K.values),B,$);return K}class eZ{constructor(Z,K){this._$parts=[],this._$disconnectableChildren=void 0,this._$template=Z,this._$parent=K}get parentNode(){return this._$parent.parentNode}get _$isConnected(){return this._$parent._$isConnected}_clone(Z){let{el:{content:K},parts:Q}=this._$template,$=(Z?.creationScope??h).importNode(K,!0);f.currentNode=$;let B=f.nextNode(),Y=0,X=0,J=Q[0];while(J!==void 0){if(Y===J.index){let q;if(J.type===BZ)q=new r(B,B.nextSibling,this,Z);else if(J.type===RZ)q=new J.ctor(B,J.name,J.strings,this,Z);else if(J.type===NZ)q=new $5(B,this,Z);this._$parts.push(q),J=Q[++X]}if(Y!==J?.index)B=f.nextNode(),Y++}return f.currentNode=h,$}_update(Z){let K=0;for(let Q of this._$parts){if(Q!==void 0)if(W&&W({kind:"set part",part:Q,value:Z[K],valueIndex:K,values:Z,templateInstance:this}),Q.strings!==void 0)Q._$setValue(Z,Q,K),K+=Q.strings.length-2;else Q._$setValue(Z[K]);K++}}}class r{get _$isConnected(){return this._$parent?._$isConnected??this.__isConnected}constructor(Z,K,Q,$){this.type=BZ,this._$committedValue=j,this._$disconnectableChildren=void 0,this._$startNode=Z,this._$endNode=K,this._$parent=Q,this.options=$,this.__isConnected=$?.isConnected??!0,this._textSanitizer=void 0}get parentNode(){let Z=y(this._$startNode).parentNode,K=this._$parent;if(K!==void 0&&Z?.nodeType===11)Z=K.parentNode;return Z}get startNode(){return this._$startNode}get endNode(){return this._$endNode}_$setValue(Z,K=this){if(this.parentNode===null)throw Error("This `ChildPart` has no `parentNode` and therefore cannot accept a value. This likely means the element containing the part was manipulated in an unsupported way outside of Lit's control such that the part's marker nodes were ejected from DOM. For example, setting the element's `innerHTML` or `textContent` can do this.");if(Z=u(this,Z,K),s(Z)){if(Z===j||Z==null||Z===""){if(this._$committedValue!==j)W&&W({kind:"commit nothing to child",start:this._$startNode,end:this._$endNode,parent:this._$parent,options:this.options}),this._$clear();this._$committedValue=j}else if(Z!==this._$committedValue&&Z!==g)this._commitText(Z)}else if(Z._$litType$!==void 0)this._commitTemplateResult(Z);else if(Z.nodeType!==void 0){if(this.options?.host===Z){this._commitText("[probable mistake: rendered a template's host in itself (commonly caused by writing ${this} in a template]"),console.warn("Attempted to render the template host",Z,"inside itself. This is almost always a mistake, and in dev mode ","we render some warning text. In production however, we'll ","render it, which will usually result in an error, and sometimes ","in the element disappearing from the DOM.");return}this._commitNode(Z)}else if(n5(Z))this._commitIterable(Z);else this._commitText(Z)}_insert(Z){return y(y(this._$startNode).parentNode).insertBefore(Z,this._$endNode)}_commitNode(Z){if(this._$committedValue!==Z){if(this._$clear(),c!==GZ){let K=this._$startNode.parentNode?.nodeName;if(K==="STYLE"||K==="SCRIPT"){let Q="Forbidden";if(K==="STYLE")Q="Lit does not support binding inside style nodes. This is a security risk, as style injection attacks can exfiltrate data and spoof UIs. Consider instead using css`...` literals to compose styles, and do dynamic styling with css custom properties, ::parts, <slot>s, and by mutating the DOM rather than stylesheets.";else Q="Lit does not support binding inside script nodes. This is a security risk, as it could allow arbitrary code execution.";throw Error(Q)}}W&&W({kind:"commit node",start:this._$startNode,parent:this._$parent,value:Z,options:this.options}),this._$committedValue=this._insert(Z)}}_commitText(Z){if(this._$committedValue!==j&&s(this._$committedValue)){let K=y(this._$startNode).nextSibling;if(this._textSanitizer===void 0)this._textSanitizer=VZ(K,"data","property");Z=this._textSanitizer(Z),W&&W({kind:"commit text",node:K,value:Z,options:this.options}),K.data=Z}else{let K=h.createTextNode("");if(this._commitNode(K),this._textSanitizer===void 0)this._textSanitizer=VZ(K,"data","property");Z=this._textSanitizer(Z),W&&W({kind:"commit text",node:K,value:Z,options:this.options}),K.data=Z}this._$committedValue=Z}_commitTemplateResult(Z){let{values:K,["_$litType$"]:Q}=Z,$=typeof Q==="number"?this._$getTemplate(Z):(Q.el===void 0&&(Q.el=i.createElement(tZ(Q.h,Q.h[0]),this.options)),Q);if(this._$committedValue?._$template===$)W&&W({kind:"template updating",template:$,instance:this._$committedValue,parts:this._$committedValue._$parts,options:this.options,values:K}),this._$committedValue._update(K);else{let B=new eZ($,this),Y=B._clone(this.options);W&&W({kind:"template instantiated",template:$,instance:B,parts:B._$parts,options:this.options,fragment:Y,values:K}),B._update(K),W&&W({kind:"template instantiated and updated",template:$,instance:B,parts:B._$parts,options:this.options,fragment:Y,values:K}),this._commitNode(Y),this._$committedValue=B}}_$getTemplate(Z){let K=iZ.get(Z.strings);if(K===void 0)iZ.set(Z.strings,K=new i(Z));return K}_commitIterable(Z){if(!DZ(this._$committedValue))this._$committedValue=[],this._$clear();let K=this._$committedValue,Q=0,$;for(let B of Z){if(Q===K.length)K.push($=new r(this._insert(l()),this._insert(l()),this,this.options));else $=K[Q];$._$setValue(B),Q++}if(Q<K.length)this._$clear($&&y($._$endNode).nextSibling,Q),K.length=Q}_$clear(Z=y(this._$startNode).nextSibling,K){this._$notifyConnectionChanged?.(!1,!0,K);while(Z!==this._$endNode){let Q=y(Z).nextSibling;y(Z).remove(),Z=Q}}setConnected(Z){if(this._$parent===void 0)this.__isConnected=Z,this._$notifyConnectionChanged?.(Z);else throw Error("part.setConnected() may only be called on a RootPart returned from render().")}}class a{get tagName(){return this.element.tagName}get _$isConnected(){return this._$parent._$isConnected}constructor(Z,K,Q,$,B){if(this.type=RZ,this._$committedValue=j,this._$disconnectableChildren=void 0,this.element=Z,this.name=K,this._$parent=$,this.options=B,Q.length>2||Q[0]!==""||Q[1]!=="")this._$committedValue=Array(Q.length-1).fill(new String),this.strings=Q;else this._$committedValue=j;this._sanitizer=void 0}_$setValue(Z,K=this,Q,$){let B=this.strings,Y=!1;if(B===void 0){if(Z=u(this,Z,K,0),Y=!s(Z)||Z!==this._$committedValue&&Z!==g,Y)this._$committedValue=Z}else{let X=Z;Z=B[0];let J,q;for(J=0;J<B.length-1;J++){if(q=u(this,X[Q+J],K,J),q===g)q=this._$committedValue[J];if(Y||=!s(q)||q!==this._$committedValue[J],q===j)Z=j;else if(Z!==j)Z+=(q??"")+B[J+1];this._$committedValue[J]=q}}if(Y&&!$)this._commitValue(Z)}_commitValue(Z){if(Z===j)y(this.element).removeAttribute(this.name);else{if(this._sanitizer===void 0)this._sanitizer=c(this.element,this.name,"attribute");Z=this._sanitizer(Z??""),W&&W({kind:"commit attribute",element:this.element,name:this.name,value:Z,options:this.options}),y(this.element).setAttribute(this.name,Z??"")}}}class Z5 extends a{constructor(){super(...arguments);this.type=B6}_commitValue(Z){if(this._sanitizer===void 0)this._sanitizer=c(this.element,this.name,"property");Z=this._sanitizer(Z),W&&W({kind:"commit property",element:this.element,name:this.name,value:Z,options:this.options}),this.element[this.name]=Z===j?void 0:Z}}class K5 extends a{constructor(){super(...arguments);this.type=G6}_commitValue(Z){W&&W({kind:"commit boolean attribute",element:this.element,name:this.name,value:!!(Z&&Z!==j),options:this.options}),y(this.element).toggleAttribute(this.name,!!Z&&Z!==j)}}class Q5 extends a{constructor(Z,K,Q,$,B){super(Z,K,Q,$,B);if(this.type=Y6,this.strings!==void 0)throw Error(`A \`<${Z.localName}>\` has a \`@${K}=...\` listener with invalid content. Event listeners in templates must have exactly one expression and no surrounding text.`)}_$setValue(Z,K=this){if(Z=u(this,Z,K,0)??j,Z===g)return;let Q=this._$committedValue,$=Z===j&&Q!==j||Z.capture!==Q.capture||Z.once!==Q.once||Z.passive!==Q.passive,B=Z!==j&&(Q===j||$);if(W&&W({kind:"commit event listener",element:this.element,name:this.name,value:Z,options:this.options,removeListener:$,addListener:B,oldListener:Q}),$)this.element.removeEventListener(this.name,this,Q);if(B)this.element.addEventListener(this.name,this,Z);this._$committedValue=Z}handleEvent(Z){if(typeof this._$committedValue==="function")this._$committedValue.call(this.options?.host??this.element,Z);else this._$committedValue.handleEvent(Z)}}class $5{constructor(Z,K,Q){this.element=Z,this.type=NZ,this._$disconnectableChildren=void 0,this._$parent=K,this.options=Q}get _$isConnected(){return this._$parent._$isConnected}_$setValue(Z){W&&W({kind:"commit to element binding",element:this.element,value:Z,options:this.options}),u(this,Z)}}var J6=N.litHtmlPolyfillSupportDevMode;J6?.(i,r);(N.litHtmlVersions??=[]).push("3.3.3");if(N.litHtmlVersions.length>1)queueMicrotask(()=>{o("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});var p=(Z,K,Q)=>{if(K==null)throw TypeError(`The container to render into may not be ${K}`);let $=l5++,B=Q?.renderBefore??K,Y=B._$litPart$;if(W&&W({kind:"begin render",id:$,value:Z,container:K,options:Q,part:Y}),Y===void 0){let X=Q?.renderBefore??null;B._$litPart$=Y=new r(K.insertBefore(l(),X),X,void 0,Q??{})}return Y._$setValue(Z),W&&W({kind:"end render",id:$,value:Z,container:K,options:Q,part:Y}),Y};p.setSanitizer=i5,p.createSanitizer=VZ,p._testOnlyClearSanitizerFactoryDoNotCallOrElse=r5;var z6=(Z,K)=>Z,SZ=!0,x=globalThis,B5;if(SZ)x.litIssuedWarnings??=new Set,B5=(Z,K)=>{if(K+=` See https://lit.dev/msg/${Z} for more information.`,!x.litIssuedWarnings.has(K)&&!x.litIssuedWarnings.has(Z))console.warn(K),x.litIssuedWarnings.add(K)};class U extends R{constructor(){super(...arguments);this.renderOptions={host:this},this.__childPart=void 0}createRenderRoot(){let Z=super.createRenderRoot();return this.renderOptions.renderBefore??=Z.firstChild,Z}update(Z){let K=this.render();if(!this.hasUpdated)this.renderOptions.isConnected=this.isConnected;super.update(Z),this.__childPart=p(K,this.renderRoot,this.renderOptions)}connectedCallback(){super.connectedCallback(),this.__childPart?.setConnected(!0)}disconnectedCallback(){super.disconnectedCallback(),this.__childPart?.setConnected(!1)}render(){return g}}U._$litElement$=!0;U[z6("finalized",U)]=!0;x.litElementHydrateSupport?.({LitElement:U});var U6=SZ?x.litElementPolyfillSupportDevMode:x.litElementPolyfillSupport;U6?.({LitElement:U});(x.litElementVersions??=[]).push("4.2.2");if(SZ&&x.litElementVersions.length>1)queueMicrotask(()=>{B5("multiple-versions","Multiple versions of Lit loaded. Loading multiple versions is not recommended.")});async function w(Z){let K=await fetch(Z,{credentials:"same-origin"});if(!K.ok)throw Error(`${Z}: ${K.status}`);return await K.json()}async function YZ(Z,K){let Q=await fetch(Z,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(K)});if(!Q.ok){let $=await Q.text();throw Error($.trim()||`${Z}: ${Q.status}`)}}async function G5(Z,K){let Q=await fetch(Z,{method:"PUT",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(K)});if(!Q.ok){let $=await Q.text();throw Error($.trim()||`${Z}: ${Q.status}`)}return await Q.json()}async function q6(Z,K){let Q=await fetch(Z,{method:"DELETE",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(K)});if(!Q.ok){let $=await Q.text();throw Error($.trim()||`${Z}: ${Q.status}`)}return await Q.json()}var O={authStatus:()=>w("/api/auth/status"),setup:(Z)=>YZ("/api/auth/setup",{password:Z}),login:(Z)=>YZ("/api/auth/login",{password:Z}),logout:()=>YZ("/api/auth/logout",{}),fleet:()=>w("/api/fleet"),system:()=>w("/api/system"),history:()=>w("/api/history"),events:(Z={})=>{let K=new URLSearchParams;if(Z.since_ms)K.set("since_ms",String(Z.since_ms));if(Z.kind)K.set("kind",Z.kind);if(Z.severity)K.set("severity",Z.severity);if(Z.inverter_uid)K.set("inverter_uid",Z.inverter_uid);if(Z.limit)K.set("limit",String(Z.limit));let Q=K.toString();return w("/api/events"+(Q?`?${Q}`:""))},getSettings:async()=>{let Z=await w("/api/settings");if(Z.error)return{error:Z.error};return{settings:{ecu_id:Z.ecu_id,mac:Z.mac,pan_override:Z.pan_override,zigbee_type:Z.zigbee_type,inverter_names:Z.inverter_names??{}}}},saveSettings:(Z)=>G5("/api/settings",Z),profiles:()=>w("/api/profiles"),overlays:()=>w("/api/overlays"),selectBase:(Z)=>YZ("/api/profiles/base",{id:Z}),saveOverlay:(Z)=>G5("/api/profiles/overlay",Z),deleteOverlay:(Z,K)=>q6("/api/profiles/overlay",{id:Z,uids:K})};function Y5(Z,K){let Q=new EventSource("/api/stream");return Q.addEventListener("fleet",($)=>{try{Z(JSON.parse($.data))}catch{}}),Q.onerror=()=>K?.(),()=>Q.close()}class j5 extends U{static properties={configured:{type:Boolean},error:{state:!0},busy:{state:!0}};constructor(){super();this.configured=!0,this.error="",this.busy=!1}static styles=H`
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
  `;async submit(Z){Z.preventDefault();let Q=this.renderRoot.querySelector("input")?.value??"";this.busy=!0,this.error="";try{if(this.configured)await O.login(Q);else await O.setup(Q);this.dispatchEvent(new CustomEvent("authed",{bubbles:!0,composed:!0}))}catch($){this.error=$.message||"failed"}finally{this.busy=!1}}render(){return G`
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
    `}}customElements.define("login-view",j5);function T(Z){if(!Number.isFinite(Z))return"";return String(Number(Z.toFixed(3)))}function S(Z){if(!Number.isFinite(Z))return"—";if(Math.abs(Z)>=1000)return`${(Z/1000).toFixed(2)} kW`;return`${Math.round(Z)} W`}function n(Z){if(!Number.isFinite(Z))return"—";let K=Math.abs(Z);if(K>=1e6)return`${(Z/1e6).toFixed(2)} MWh`;if(K>=1000)return`${(Z/1000).toFixed(2)} kWh`;return`${Math.round(Z)} Wh`}function v(Z){return Number.isFinite(Z)?`${Z.toFixed(0)}%`:"—"}function t(Z){return Z>0?`${Z.toFixed(1)} V`:"—"}function jZ(Z){return Z>0?`${Z.toFixed(2)} Hz`:"—"}function X5(Z){return Number.isFinite(Z)?`${Z.toFixed(2)} A`:"—"}function XZ(Z){if(!(Z>0))return"idle";if(Z<40)return"low";if(Z<85)return"mid";return"high"}function J5(Z){if(!Number.isFinite(Z)||Z<0)return"—";if(Z<60)return`${Math.round(Z)}s ago`;if(Z<3600)return`${Math.round(Z/60)}m ago`;return`${Math.round(Z/3600)}h ago`}function PZ(Z){return Z.replace(/_/g," ").replace(/\b\w/g,(K)=>K.toUpperCase())}function JZ(Z){if(!Z)return[];return Object.keys(Z).filter((K)=>Z[K]).map(PZ)}function zZ(Z){if(!Z)return"—";return new Date(Z).toLocaleString(void 0,{hour12:!1})}function z5(Z){let K=(Z||"").toLowerCase();if(K==="error"||K==="critical"||K==="crit"||K==="fault")return"err";if(K==="warn"||K==="warning")return"warn";return"info"}class U5 extends U{static properties={power:{type:Number},cap:{type:Number}};constructor(){super();this.power=0,this.cap=0}static styles=H`
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
  `;pct(){if(!(this.cap>0))return 0;return Math.max(0,Math.min(100,this.power/this.cap*100))}render(){let Z=this.pct(),K=XZ(Z),Q=90,$=Math.PI*90,B=$*(1-Z/100);return G`
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
          <div class="big">${S(this.power)}</div>
          <div class="sub">${v(Z)} of ${S(this.cap)}</div>
        </div>
      </div>
    `}}customElements.define("fleet-gauge",U5);class q5 extends U{static properties={label:{type:String},value:{type:String},sub:{type:String}};constructor(){super();this.label="",this.value="",this.sub=""}static styles=H`
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
    `}}customElements.define("stat-card",q5);class H5 extends U{static properties={inverter:{attribute:!1},name:{type:String},profile:{type:String}};constructor(){super();this.name="",this.profile=""}static styles=H`
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
  `;render(){let Z=this.inverter;if(!Z)return j;let K=XZ(Z.load_pct),Q=JZ(Z.faults),$=Math.max(0,Math.min(100,Z.load_pct));return G`
      <div class="head">
        <div>
          <div class="model">${this.name||Z.model||"unknown"}</div>
          <div class="uid">${this.name?`${Z.model} · ${Z.uid}`:Z.uid}</div>
          ${this.profile?G`<div class="profile" title="Local Site profile active">⚙ ${this.profile}</div>`:j}
        </div>
        <div class="state">
          <span class="dot ${Z.online?"on":"off"}"></span>
          ${Z.online?"online":"offline"} · ${J5(Z.age_s)}
        </div>
      </div>

      <div class="power">
        <span class="pw">${S(Z.active_power_w)}</span>
        <span class="cap">/ ${Z.nameplate_w} W · ${v(Z.load_pct)}</span>
      </div>
      <div class="bar"><div class="fill ${K}" style="width:${$}%"></div></div>

      <div class="metrics">
        <div class="metric"><div class="k">Grid</div><div class="v">${t(Z.grid_v)}</div></div>
        <div class="metric"><div class="k">Freq</div><div class="v">${jZ(Z.freq_hz)}</div></div>
        <div class="metric"><div class="k">RSSI / LQI</div><div class="v">${Z.rssi} / ${Z.lqi}</div></div>
      </div>

      ${Z.panels?.length?G`<div class="panels">
            ${Z.panels.map((B)=>G`<div class="panel">
                <div class="pi">DC ${B.index+1}</div>
                <div class="pw">${S(B.w)}</div>
                <div>${t(B.dc_v)} · ${X5(B.dc_a)}</div>
              </div>`)}
          </div>`:j}

      ${Q.length?G`<div class="chips">
            ${Q.map((B)=>G`<span class="chip">${B}</span>`)}
          </div>`:j}
    `}}customElements.define("inverter-card",H5);class M5 extends U{static properties={system:{attribute:!1}};constructor(){super();this.system=null}static styles=H`
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
  `;idRow(Z,K){return K?G`<div class="k">${Z}</div><div class="v">${K}</div>`:j}clients(){let Z=new Map;for(let K of this.system?.peers??[]){let Q=Z.get(K.backend)??{backend:K.backend,version:K.version,controller:!1,conns:0};if(Q.conns++,Q.controller=Q.controller||K.controller,K.version)Q.version=K.version;Z.set(K.backend,Q)}return[...Z.values()].sort((K,Q)=>K.backend.localeCompare(Q.backend))}render(){let Z=this.system,K=Z?.ecu,Q=this.clients(),$=!!(K&&(K.ecu_id||K.hostname));return G`
      ${$?G`<div class="id">
            ${this.idRow("ECU ID",K.ecu_id)}
            ${this.idRow("Host",K.hostname)}
          </div>`:j}

      <div class="peers">
        ${Q.length?Q.map((B)=>G`<div class="peer">
                <span class="dot on"></span>
                <span class="name">${B.backend||"(unnamed)"}</span>
                ${B.controller?G`<span class="role ctl">ctrl</span>`:j}
                ${B.conns>1?G`<span class="role">${B.conns} conns</span>`:j}
                <span class="ver">${B.version||""}</span>
              </div>`):G`<div class="empty">No peers connected.</div>`}
      </div>

      ${Z?.status_error?G`<div class="warn">⚠ ${Z.status_error}</div>`:j}
    `}}customElements.define("ecu-clients-card",M5);function H6(Z,K,Q){if(Z.length<2)return{line:"",area:"",max:0};let $=Z[0].t,B=Math.max(1,Z[Z.length-1].t-$),Y=Math.max(1,...Z.map((k)=>k.w)),X=(k)=>[(k.t-$)/B*K,Q-k.w/Y*Q],J="";for(let k=0;k<Z.length;k++){let[_,F]=X(Z[k]);J+=`${k===0?"M":"L"}${_.toFixed(1)} ${F.toFixed(1)} `}let[q]=X(Z[0]),[M]=X(Z[Z.length-1]),z=`${J}L${M.toFixed(1)} ${Q} L${q.toFixed(1)} ${Q} Z`;return{line:J.trim(),area:z,max:Y}}var UZ=600,e=160;class k5 extends U{static properties={points:{attribute:!1},hoverIdx:{state:!0}};constructor(){super();this.points=[],this.hoverIdx=-1}static styles=H`
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
  `;onMove=(Z)=>{let K=this.points.length;if(K<2)return;let $=Z.currentTarget.clientWidth||1,B=Math.min(1,Math.max(0,Z.offsetX/$));this.hoverIdx=Math.round(B*(K-1))};onLeave=()=>{this.hoverIdx=-1};render(){let Z=this.points??[];if(Z.length<2)return G`<div class="empty">Collecting power history…</div>`;let{line:K,area:Q,max:$}=H6(Z,UZ,e),B=Z[Z.length-1].w,Y=this.hoverIdx,X=Y>=0&&Y<Z.length,J=Z[0].t,q=Math.max(1,Z[Z.length-1].t-J),M=X?(Z[Y].t-J)/q*UZ:0,z=X?e-Z[Y].w/$*e:0;return G`
      <div class="wrap">
        <svg
          viewBox="0 0 ${UZ} ${e}"
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
          ${L`<path class="area" d=${Q} />`}
          ${L`<path class="line" d=${K} />`}
          ${X?L`<line class="cross" x1=${M} y1="0" x2=${M} y2=${e} /><circle class="cursor" cx=${M} cy=${z} r="3.5" />`:j}
        </svg>
        ${X?G`<div class="tip" style="left:${M/UZ*100}%; top:${z}px">
              <span class="w">${S(Z[Y].w)}</span>
              <span class="t">· ${zZ(Z[Y].t)}</span>
            </div>`:j}
      </div>
      <div class="labels">
        <span>now <span class="cur">${S(B)}</span></span>
        <span>peak ${S($)}</span>
      </div>
    `}}customElements.define("power-chart",k5);class F5 extends U{static properties={fleet:{attribute:!1},system:{attribute:!1},names:{attribute:!1},profiles:{attribute:!1},history:{state:!0}};timer=null;constructor(){super();this.fleet=null,this.system=null,this.names={},this.profiles={},this.history=[]}connectedCallback(){super.connectedCallback(),this.loadHistory(),this.timer=setInterval(()=>void this.loadHistory(),60000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async loadHistory(){try{this.history=await O.history()}catch{}}chartPoints(){if(!this.fleet)return this.history;return[...this.history,{t:Date.now(),w:this.fleet.active_power_w}]}static styles=H`
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
        <stat-card label="Today" value=${n(Z.today_wh)}></stat-card>
        <stat-card label="This month" value=${n(Z.month_wh)}></stat-card>
        <stat-card label="This year" value=${n(Z.year_wh)}></stat-card>
        <stat-card label="Lifetime" value=${n(Z.lifetime_wh)}></stat-card>
      </div>

      <h2>Inverters</h2>
      ${Z.inverters.length?G`<div class="cards">
            ${Z.inverters.map((K)=>G`<inverter-card
                .inverter=${K}
                .name=${this.names?.[K.uid]??""}
                .profile=${this.profiles?.[K.uid]??""}
              ></inverter-card>`)}
          </div>`:G`<div class="empty">No inverters discovered yet.</div>`}
      ${j}
    `}}customElements.define("dashboard-view",F5);class W5 extends U{static properties={fleet:{attribute:!1},names:{attribute:!1}};constructor(){super();this.fleet=null,this.names={}}rename(Z,K){let Q=K.target.value;this.dispatchEvent(new CustomEvent("rename",{detail:{uid:Z,name:Q},bubbles:!0,composed:!0}))}static styles=H`
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
              <td class="num">${S(K.active_power_w)} / ${K.nameplate_w} W</td>
              <td class="num">${v(K.load_pct)}</td>
              <td class="num">${t(K.grid_v)}</td>
              <td class="num">${jZ(K.freq_hz)}</td>
              <td class="num">${K.panels?.length??0}</td>
              <td class="num ${Q?"fault":""}">${Q||"—"}</td>
            </tr>`})}
        </tbody>
      </table>
    `}}customElements.define("inverters-view",W5);class _5 extends U{static properties={fleet:{attribute:!1}};constructor(){super();this.fleet=null}static styles=H`
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
  `;alarms(){let Z=[];for(let K of this.fleet?.inverters??[]){for(let Q of JZ(K.faults))Z.push({uid:K.uid,model:K.model,label:Q,severity:"fault"});if(!K.online)Z.push({uid:K.uid,model:K.model,label:"Inverter offline",severity:"warning"})}return Z}render(){let Z=this.alarms();if(Z.length===0)return G`<div class="ok"><div class="big">✓ No active alarms</div><div>All inverters reporting healthy.</div></div>`;return G`${Z.map((K)=>G`<div class="row ${K.severity}">
        <span class="sev">${K.severity}</span>
        <span class="label">${K.label} <span style="color:var(--muted)">· ${K.model||"?"}</span></span>
        <span class="uid">${K.uid}</span>
      </div>`)}`}}customElements.define("alarms-view",_5);class A5 extends U{static properties={events:{attribute:!1}};constructor(){super();this.events=[]}static styles=H`
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
              <td><span class="sev ${z5(Z.severity)}">${Z.severity}</span></td>
              <td>${PZ(Z.kind)}</td>
              <td class="uid">${Z.inverter_uid||"—"}</td>
              <td class="detail">${Z.detail||(Z.raw_hex?Z.raw_hex:j)}</td>
            </tr>`)}
        </tbody>
      </table>
    `}}customElements.define("events-table",A5);class O5 extends U{static properties={events:{state:!0},error:{state:!0},loading:{state:!0}};timer=null;constructor(){super();this.events=[],this.error="",this.loading=!1}static styles=H`
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
  `;connectedCallback(){super.connectedCallback(),this.load(),this.timer=setInterval(()=>void this.load(),15000)}disconnectedCallback(){if(super.disconnectedCallback(),this.timer)clearInterval(this.timer);this.timer=null}async load(){this.loading=!0;try{let Z=await O.events({limit:200});this.events=Z.events??[],this.error=Z.error??""}catch(Z){this.error=Z.message}finally{this.loading=!1}}render(){return G`
      <div class="bar">
        <span class="count">${this.events.length} event(s)${this.loading?" · refreshing…":""}</span>
        <button @click=${()=>void this.load()}>Refresh</button>
      </div>
      ${this.error?G`<div class="err">⚠ ${this.error}</div>`:j}
      <div class="panel"><events-table .events=${this.events}></events-table></div>
    `}}customElements.define("events-view",O5);class I5 extends U{static properties={profiles:{attribute:!1},activeBase:{attribute:!1},reconcilerReady:{attribute:!1},busy:{attribute:!1},selected:{state:!0}};constructor(){super();this.profiles=[],this.activeBase="",this.reconcilerReady=!0,this.busy=!1,this.selected=""}static styles=H`
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
          ${this.activeBase?G` <strong>${this.activeBase}</strong>${K?G` <span class="muted">(${K.vnom_v} V · ${K.point_count} pts)</span>`:j}`:G` <span class="none">none selected</span>`}
        </div>

        <label>
          Base profile
          <select id="profile" .value=${Z} @change=${this.onChange} ?disabled=${this.busy}>
            ${this.activeBase?j:G`<option value="" disabled selected>Select a profile…</option>`}
            ${this.profiles.map(($)=>G`<option value=${$.id} ?selected=${$.id===Z}>${this.labelFor($)}</option>`)}
          </select>
        </label>

        <div class="actions">
          <button class="apply" @click=${this.apply} ?disabled=${!Q}>
            ${this.busy?"Applying…":"Apply"}
          </button>
          ${!this.reconcilerReady?G`<span class="hint">reconciler not ready</span>`:Z&&Z!==this.activeBase?G`<span class="hint">applies to all inverters</span>`:j}
        </div>
      </div>
    `}}customElements.define("grid-profile-form",I5);var T5={AC:{label:"Undervoltage trip — stage 2",desc:"Disconnect when AC voltage drops to this lower-stage level."},AQ:{label:"Undervoltage trip — deep",desc:"Disconnect quickly when voltage falls this far below nominal."},AH:{label:"Undervoltage trip — fast",desc:"Fast disconnect on a severe undervoltage."},AD:{label:"Overvoltage trip — slow",desc:"Disconnect when AC voltage rises above this (slower stage)."},AY:{label:"Overvoltage trip — slow (stage 2)",desc:"Second slower overvoltage disconnect threshold."},AB:{label:"10-minute mean overvoltage",desc:"Trips if the 10-minute average voltage exceeds this (EN 50549 sustained-overvoltage limit)."},AI:{label:"Overvoltage trip — fast",desc:"Fast disconnect on a severe overvoltage."},AE:{label:"Underfrequency trip — slow",desc:"Disconnect when grid frequency falls below this (slower stage)."},AJ:{label:"Underfrequency trip — fast",desc:"Fast disconnect on a severe underfrequency."},AF:{label:"Overfrequency trip — slow",desc:"Disconnect when grid frequency rises above this (slower stage)."},AK:{label:"Overfrequency trip — fast",desc:"Fast disconnect on a severe overfrequency."},BB:{label:"Undervoltage 1 — clearance time",desc:"How long the undervoltage condition must persist before tripping."},BD:{label:"Undervoltage 2 — clearance time",desc:"Clearance delay for the second undervoltage stage."},BC:{label:"Overvoltage 1 — clearance time",desc:"How long the overvoltage condition must persist before tripping."},BE:{label:"Overvoltage 2 — clearance time",desc:"Clearance delay for the second overvoltage stage."},BH:{label:"Underfrequency 1 — clearance time",desc:"Clearance delay for the first underfrequency stage."},BJ:{label:"Underfrequency 2 — clearance time",desc:"Clearance delay for the second underfrequency stage."},BI:{label:"Overfrequency 1 — clearance time",desc:"Clearance delay for the first overfrequency stage."},BK:{label:"Overfrequency 2 — clearance time",desc:"Clearance delay for the second overfrequency stage."},BN:{label:"Enter-service voltage — lower",desc:"Voltage must be above this before the inverter reconnects."},BO:{label:"Enter-service voltage — upper",desc:"Voltage must be below this before the inverter reconnects."},BP:{label:"Enter-service frequency — lower",desc:"Frequency must be above this before the inverter reconnects."},BQ:{label:"Enter-service frequency — upper",desc:"Frequency must be below this before the inverter reconnects."},AG:{label:"Grid-recovery delay",desc:"Wait time after the grid is healthy before reconnecting."},AS:{label:"Power ramp time",desc:"Time taken to ramp output back up after reconnecting."},CV:{label:"Curtailment enable (droop)",desc:"Enables the over-frequency droop power reduction (0 = off, 1 = on)."},CA:{label:"Curtailment start (droop deadband)",desc:"Over-frequency droop: power reduction begins at this frequency (deadband end)."},DD:{label:"Curtailment slope (droop)",desc:"Over-frequency droop gradient: % of rated power reduced per Hz above the start."},CG:{label:"Curtailment response time (droop)",desc:"Filter/response time of the droop control loop."},DH:{label:"Under-freq curve — low",desc:"Legacy frequency-Watt curve: lower frequency point of the under-frequency response."},DI:{label:"Under-freq curve — high",desc:"Legacy frequency-Watt curve: upper frequency point of the under-frequency response."},CB:{label:"Over-freq curve — start",desc:"Legacy frequency-Watt curve: over-frequency power reduction begins at this frequency."},CC:{label:"Over-freq curve — end",desc:"Legacy frequency-Watt curve: over-frequency reduction reaches its limit at this frequency."}},C5={DERFreqDroop:{label:"Frequency-Watt droop",tip:"Linearly reduces active power as frequency rises above a deadband — over-frequency curtailment (SunSpec DERFreqDroop, model 711)."},CrvSet:{label:"Frequency-Watt curve",tip:"Legacy point-based power-versus-frequency response curve (model 134)."},MustTrip:{label:"Trip thresholds",tip:"Voltage and frequency limits that disconnect the inverter from the grid when crossed (protection trips)."},DEREnterService:{label:"Enter service",tip:"The voltage/frequency window and timing the inverter must satisfy before (re)connecting after a trip."}},yZ=["DERFreqDroop","CrvSet","MustTrip","DEREnterService"],V5=new Set(["MustTrip","DEREnterService"]);function M6(Z,K){if(!Z)return K;return Z.replace(/_/g," ").replace(/\b\w/g,(Q)=>Q.toUpperCase())}function D5(Z,K){return T5[Z]?.label??M6(K??"",Z)}function R5(Z){return T5[Z]?.desc??""}function EZ(Z,K){let Q=[];for(let $ of Z){let B=K($.left),Y=K($.right);if(B!==void 0&&Y!==void 0&&!(B<Y))Q.push($.message)}return Q}class N5 extends U{static properties={deadband:{type:Number},slope:{type:Number},trip:{type:Number},nominal:{type:Number}};constructor(){super();this.nominal=50}static styles=H`
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
  `;render(){let Z=this.deadband,K=this.slope,Q=this.trip,$=this.nominal;if(Z===void 0||K===void 0||K<=0)return G`<div class="empty">Set the curtailment start frequency and slope to preview the curve.</div>`;let B=Z+100/K,Y=$-0.3,X=Math.max(Q??0,B,Z+1.5,$+1.5)+0.2,J=480,q=170,M=36,z=12,k=10,_=24,F=(I)=>M+(I-Y)/(X-Y)*(J-M-z),C=(I)=>k+(100-I)/100*(q-k-_),A=Math.min(B,X),qZ=Math.max(0,100-K*(A-Z)),HZ=[[Y,100],[Z,100],[A,qZ],...B<X?[[X,0]]:[]].map(([I,w5])=>`${F(I).toFixed(1)},${C(w5).toFixed(1)}`).join(" "),MZ=[];for(let I=Math.ceil(Y*2)/2;I<=X;I+=0.5)MZ.push(I);return G`
      <svg viewBox="0 0 ${J} ${q}" role="img" aria-label="Frequency-Watt curtailment curve">
        ${[0,50,100].map((I)=>L`<line class="grid" x1=${M} y1=${C(I)} x2=${J-z} y2=${C(I)} />
            <text x=${M-4} y=${C(I)+3} text-anchor="end">${I}%</text>`)}
        ${MZ.map((I)=>L`<text x=${F(I)} y=${q-_+12} text-anchor="middle">${I.toFixed(1)}</text>`)}
        <line class="frame" x1=${M} y1=${k} x2=${M} y2=${q-_} />
        <line class="frame" x1=${M} y1=${q-_} x2=${J-z} y2=${q-_} />
        <line class="dead" x1=${F(Z)} y1=${k} x2=${F(Z)} y2=${q-_} />
        <text class="lbl" x=${F(Z)} y=${k+8} text-anchor="middle">start ${T(Z)} Hz</text>
        ${B<=X?L`<line class="dead" x1=${F(B)} y1=${k} x2=${F(B)} y2=${q-_} />
              <text class="lbl" x=${F(B)} y=${k+8} text-anchor="middle">0% at ${T(B)} Hz</text>`:j}
        ${Q!==void 0&&Q>=Y&&Q<=X?L`<line class="trip" x1=${F(Q)} y1=${k} x2=${F(Q)} y2=${q-_} />
              <text x=${F(Q)} y=${q-_-4} text-anchor="middle" fill="var(--err)">trip ${T(Q)} Hz</text>`:j}
        <polyline class="curve" points=${HZ} />
        <text x=${J/2} y=${q-2} text-anchor="middle">Power vs frequency · slope ${T(K)} %Pref/Hz</text>
      </svg>
    `}}customElements.define("freq-watt-chart",N5);class L5 extends U{static properties={unit:{type:String},nominal:{type:Number},markers:{attribute:!1}};constructor(){super();this.unit="",this.markers=[]}static styles=H`
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
  `;render(){let Z=(this.markers??[]).filter((A)=>Number.isFinite(A.value));if(!Z.length)return G`<div class="empty">No thresholds set.</div>`;let K=Z.map((A)=>A.value).concat(this.nominal!==void 0?[this.nominal]:[]),Q=Math.min(...K),$=Math.max(...K),B=($-Q)*0.14||1;Q-=B,$+=B;let Y=480,X=70,J=10,q=10,M=34,z=(A)=>J+(A-Q)/($-Q)*(Y-J-q),k=Z.filter((A)=>A.kind==="under").map((A)=>A.value),_=Z.filter((A)=>A.kind==="over").map((A)=>A.value),F=k.length?Math.max(...k):Q,C=_.length?Math.min(..._):$;return G`
      <svg viewBox="0 0 ${Y} ${X}" role="img" aria-label="Trip thresholds">
        ${C>F?L`<rect class="band" x=${z(F)} y=${M-8} width=${z(C)-z(F)} height=16 />`:j}
        <line class="axis" x1=${J} y1=${M} x2=${Y-q} y2=${M} />
        ${this.nominal!==void 0?L`<line class="nom" x1=${z(this.nominal)} y1=${M-9} x2=${z(this.nominal)} y2=${M+9} />
              <text x=${z(this.nominal)} y=${M+20} text-anchor="middle" fill="var(--ok)">${T(this.nominal)} ${this.unit}</text>`:j}
        ${Z.map((A,qZ)=>{let HZ=A.kind,I=qZ%2===0?M-12:M+22;return L`<line class=${HZ} x1=${z(A.value)} y1=${M-7} x2=${z(A.value)} y2=${M+7} />
            <text x=${z(A.value)} y=${I} text-anchor="middle">${A.label} ${T(A.value)}</text>`})}
      </svg>
    `}}customElements.define("trip-line",L5);class S5 extends U{static properties={params:{attribute:!1},inverters:{attribute:!1},defaults:{attribute:!1},rules:{attribute:!1},profile:{attribute:!1},names:{attribute:!1},busy:{attribute:!1},editing:{attribute:!1},name:{state:!0},selectedUids:{state:!0},values:{state:!0},localError:{state:!0}};constructor(){super();this.params=[],this.inverters=[],this.defaults={},this.rules=[],this.profile=null,this.names={},this.busy=!1,this.editing=!1,this.name="",this.selectedUids=[],this.values={},this.localError=""}static styles=H`
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
  `;willUpdate(Z){if(Z.has("profile")){let K=this.profile;this.name=K?.id??"",this.selectedUids=[...K?.uids??[]];let Q={};for(let $ of K?.points??[])Q[$.aps_code]=String($.value);this.values=Q,this.localError=""}}effectiveWritable(){if(!this.selectedUids.length)return new Set;let Z=this.selectedUids.map((Q)=>new Set(this.inverters.find(($)=>$.uid===Q)?.writable_codes??[])),K=Z[0];for(let Q of Z.slice(1))K=new Set([...K].filter(($)=>Q.has($)));return K}targetDefault(Z){let K=this.defaults[Z];if(K)return{value:K.value,source:"base"};if(!this.selectedUids.length)return;let Q;for(let $ of this.selectedUids){let B=this.inverters.find((Y)=>Y.uid===$)?.current?.[Z];if(B===void 0)return;if(Q===void 0)Q=B;else if(Math.abs(B-Q)>0.000001)return}return Q===void 0?void 0:{value:Q,source:"inverter"}}effectiveValue(Z){let K=(this.values[Z]??"").trim();if(K!==""&&!Number.isNaN(Number(K)))return Number(K);return this.targetDefault(Z)?.value}isOverride(Z){let K=(this.values[Z]??"").trim();if(K===""||Number.isNaN(Number(K)))return!1;let Q=this.targetDefault(Z);return!Q||Number(K)!==Q.value}prefill(Z){if((this.values[Z]??"").trim()!=="")return;let K=this.targetDefault(Z);if(K)this.setValue(Z,T(K.value))}outOfRange(Z){let K=(this.values[Z]??"").trim();if(K===""||Number.isNaN(Number(K)))return!1;let Q=this.defaults[Z];if(!Q)return!1;let $=Number(K);return Q.min!==void 0&&$<Q.min||Q.max!==void 0&&$>Q.max}label(Z){return this.names[Z.uid]||Z.model||Z.uid}toggleTarget(Z,K){this.selectedUids=K?[...this.selectedUids,Z]:this.selectedUids.filter((Q)=>Q!==Z)}setValue(Z,K){this.values={...this.values,[Z]:K}}groups(){let Z={};for(let Q of this.params)(Z[Q.group]??=[]).push(Q);return[...yZ,...Object.keys(Z).filter((Q)=>!yZ.includes(Q))].filter((Q)=>Z[Q]?.length).map((Q)=>[Q,Z[Q]])}save=()=>{let Z=this.effectiveWritable(),K=this.params.filter(($)=>Z.has($.aps_code)&&this.isOverride($.aps_code)).map(($)=>({aps_code:$.aps_code,value:Number(this.values[$.aps_code])}));if(!this.name.trim())return void(this.localError="Profile name is required.");if(!this.selectedUids.length)return void(this.localError="Select at least one target inverter.");if(!K.length)return void(this.localError="Change at least one parameter from its default.");if(EZ(this.rules,($)=>this.effectiveValue($)).length)return void(this.localError="Resolve the conflicts before saving.");this.localError="";let Q={id:this.name.trim(),uids:this.selectedUids,points:K};this.dispatchEvent(new CustomEvent("save",{detail:Q,bubbles:!0,composed:!0}))};cancel=()=>this.dispatchEvent(new CustomEvent("cancel",{bubbles:!0,composed:!0}));trips(Z){let K=[];for(let[Q,$]of Z){let B=this.effectiveValue(Q);if(B!==void 0)K.push({value:B,label:Q,kind:$})}return K}vizFor(Z){if(Z==="DERFreqDroop")return G`<freq-watt-chart
        .deadband=${this.effectiveValue("CA")}
        .slope=${this.effectiveValue("DD")}
        .trip=${this.effectiveValue("AF")}
        .nominal=${50}
      ></freq-watt-chart>`;if(Z==="CrvSet"){let K=this.trips([["DH","under"],["DI","under"],["CB","over"],["CC","over"]]);return K.length?G`<trip-line unit="Hz" .nominal=${50} .markers=${K}></trip-line>`:j}if(Z==="MustTrip"){let K=this.trips([["AC","under"],["AQ","under"],["AH","under"],["AD","over"],["AY","over"],["AB","over"],["AI","over"]]),Q=this.trips([["AE","under"],["AJ","under"],["AF","over"],["AK","over"]]);return G`
        ${K.length?G`<trip-line unit="V" .nominal=${230} .markers=${K}></trip-line>`:j}
        ${Q.length?G`<trip-line unit="Hz" .nominal=${50} .markers=${Q}></trip-line>`:j}
      `}return j}renderRow(Z,K){let Q=K.has(Z.aps_code),$=this.targetDefault(Z.aps_code),B=this.defaults[Z.aps_code],Y=(this.values[Z.aps_code]??"").trim(),X=this.isOverride(Z.aps_code),J=Q&&this.outOfRange(Z.aps_code),q=Q?this.values[Z.aps_code]??"":$?T($.value):"";return G`<tr class="${Q?"":"off"} ${X?"over":""}">
      <td>
        <div class="plabel">
          ${D5(Z.aps_code,Z.long_name)}
          ${X?G`<span class="otag">overridden</span>`:j}
          ${!Q&&$?G`<span class="rotag">read-only</span>`:j}
        </div>
        <div class="pdesc">${R5(Z.aps_code)}</div>
      </td>
      <td class="pcode">${Z.aps_code}</td>
      <td class="def">
        ${$?G`${T($.value)} ${Z.unit}${$.source==="inverter"?G` <span class="src" title="from the inverter's current value">inv</span>`:j}`:"—"}
      </td>
      <td class="val">
        <input
          type="number" step="any" ?disabled=${!Q}
          .value=${q}
          placeholder=${$?T($.value):Q?"—":"n/a"}
          @focus=${()=>this.prefill(Z.aps_code)}
          @input=${(M)=>this.setValue(Z.aps_code,M.target.value)}
        />
        <span class="unit">${Z.unit}</span>
        ${Q&&Y!==""?G`<button class="clear" title="Clear override" @click=${()=>this.setValue(Z.aps_code,"")}>↺</button>`:j}
        ${J?G`<span class="warn">⚠ outside base range${B?.min!==void 0?` (${T(B.min)}–${T(B.max)} ${Z.unit})`:""}</span>`:j}
      </td>
    </tr>`}render(){let Z=this.effectiveWritable(),K=this.selectedUids.length>0,Q=K?EZ(this.rules,($)=>this.effectiveValue($)):[];return G`
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
                  </div>`:j}

              ${this.groups().map(([$,B])=>{let Y=C5[$];return G`<details class="group" ?open=${!V5.has($)}>
                  <summary>
                    <span class="gname">${Y?.label??$}</span>
                    <span class="gcount">${B.length} setting${B.length===1?"":"s"}</span>
                  </summary>
                  ${Y?.tip?G`<div class="gdesc">${Y.tip}</div>`:j}
                  <div class="viz">${this.vizFor($)}</div>
                  <table>
                    <thead><tr><th>Setting</th><th>Code</th><th>Default</th><th>Override</th></tr></thead>
                    <tbody>${B.map((X)=>this.renderRow(X,Z))}</tbody>
                  </table>
                </details>`})}

              ${this.selectedUids.length>1?G`<div class="hint">Greyed rows are not writable on every selected target.</div>`:j}
            `}

        ${this.localError?G`<div class="err">⚠ ${this.localError}</div>`:j}

        <div class="actions">
          <button class="save" @click=${this.save} ?disabled=${this.busy||Q.length>0}>
            ${this.busy?"Applying…":"Save & apply"}
          </button>
          <button class="cancel" @click=${this.cancel} ?disabled=${this.busy}>Cancel</button>
          <span class="hint">${Q.length?"resolve conflicts to save":"applies to the selected inverters"}</span>
        </div>
      </div>
    `}}customElements.define("local-site-profile-form",S5);class P5 extends U{static properties={data:{state:!0},names:{state:!0},error:{state:!0},notice:{state:!0},baseBusy:{state:!0},overlayBusy:{state:!0},editing:{state:!0},editingExisting:{state:!0}};constructor(){super();this.data=null,this.names={},this.error="",this.notice="",this.baseBusy=!1,this.overlayBusy=!1,this.editing=null,this.editingExisting=!1}static styles=H`
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
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){try{let[Z,K]=await Promise.all([O.profiles(),O.getSettings()]);this.data=Z,this.error=Z.error??"",this.names=K.settings?.inverter_names??{}}catch(Z){this.error=Z.message}}invName(Z){if(this.names[Z])return this.names[Z];return this.data?.inverters.find((Q)=>Q.uid===Z)?.model||Z}onSelectBase=async(Z)=>{let K=Z.detail;if(!window.confirm(`Apply base grid profile "${K}" to every inverter? This writes grid-protection settings across the whole fleet.`))return;this.baseBusy=!0,this.notice="",this.error="";try{await O.selectBase(K),await this.load(),this.notice=`Base profile "${K}" applied.`}catch(Q){this.error=Q.message}finally{this.baseBusy=!1}};newProfile(){this.editing={id:"",uids:[],points:[]},this.editingExisting=!1,this.notice="",this.error=""}editProfile(Z){this.editing=Z,this.editingExisting=!0,this.notice="",this.error=""}onCancelEdit=()=>{this.editing=null};exportProfile(Z){let K={id:Z.id,uids:Z.uids,points:Z.points.map((Y)=>({aps_code:Y.aps_code,value:Y.value}))},Q=new Blob([JSON.stringify(K,null,2)],{type:"application/json"}),$=URL.createObjectURL(Q),B=document.createElement("a");B.href=$,B.download=`${Z.id||"profile"}.json`,B.click(),URL.revokeObjectURL($)}triggerImport=()=>{this.shadowRoot?.querySelector("#importfile")?.click()};onImportFile=async(Z)=>{let K=Z.target,Q=K.files?.[0];if(K.value="",!Q)return;try{let $=JSON.parse(await Q.text());if(!$||!Array.isArray($.points))throw Error("not a profile (no points)");let B={id:typeof $.id==="string"?$.id:"",uids:Array.isArray($.uids)?$.uids.filter((Y)=>typeof Y==="string"):[],points:$.points.filter((Y)=>typeof Y?.aps_code==="string"&&typeof Y?.value==="number").map((Y)=>({aps_code:Y.aps_code,value:Y.value}))};this.editing=B,this.editingExisting=!1,this.error="",this.notice=`Imported "${B.id||"profile"}" — review the targets and values, then Save.`}catch($){this.error="Import failed: "+$.message}};onSaveOverlay=async(Z)=>{let K=Z.detail;if(!window.confirm(`Apply Local Site profile "${K.id}" to ${K.uids.length} inverter(s)? This writes grid-protection parameters to each.`))return;this.overlayBusy=!0,this.notice="",this.error="";try{let Q=await O.saveOverlay(K);this.editing=null,await this.load(),this.reportResults(K.id,Q.results)}catch(Q){this.error=Q.message}finally{this.overlayBusy=!1}};deleteProfile=async(Z)=>{if(!window.confirm(`Delete Local Site profile "${Z.id}" and clear it from ${Z.uids.length} inverter(s)?`))return;this.overlayBusy=!0,this.notice="",this.error="";try{let K=await O.deleteOverlay(Z.id,Z.uids);if(this.editing?.id===Z.id)this.editing=null;await this.load(),this.reportResults(Z.id,K.results,"cleared")}catch(K){this.error=K.message}finally{this.overlayBusy=!1}};reportResults(Z,K,Q="applied"){let $=K.filter((B)=>!B.ok);if($.length===0)this.notice=`Profile "${Z}" ${Q} to ${K.length} inverter(s).`;else{let B=Q==="cleared"?"clearing":"applying",Y=$.map((X)=>`${this.invName(X.uid)}: ${X.error||"unconfirmed"}`).join("; ");this.notice=`Profile "${Z}" saved on the ECU, but ${B} was not confirmed on ${$.length} of ${K.length} inverter(s) (offline?) — ${Y}`}}renderBase(){let Z=this.data?.base;return G`
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
              </div>`:j}
        </div>
        <input id="importfile" type="file" accept=".json,application/json" hidden @change=${this.onImportFile} />

        ${this.editing!==null?G`<local-site-profile-form
              .params=${Z?.params??[]}
              .inverters=${Z?.inverters??[]}
              .defaults=${Z?.base_defaults??{}}
              .rules=${Z?.conflict_rules??[]}
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
            ${K.points.map((Q)=>G`<span class="chip">${Q.aps_code} = ${T(Q.value)}${Q.unit?` ${Q.unit}`:""}</span>`)}
          </div>
          <div class="cardactions">
            <button @click=${()=>this.editProfile(K)}>Edit</button>
            <button @click=${()=>this.exportProfile(K)}>Export</button>
            <button class="del" @click=${()=>this.deleteProfile(K)}>Delete</button>
          </div>
        </div>`)}
    </div>`}render(){return G`
      ${this.notice?G`<div class="banner ok">${this.notice}</div>`:j}
      ${this.error?G`<div class="banner err">⚠ ${this.error}</div>`:j}
      ${this.data===null?G`<div class="panel"><div class="loading">Loading…</div></div>`:G`<div class="cols">
            <div>${this.renderLocalSite()}</div>
            <div>${this.renderBase()}</div>
          </div>`}
    `}}customElements.define("profiles-view",P5);class y5 extends U{static properties={settings:{attribute:!1}};constructor(){super();this.settings={ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}static styles=H`
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
    `}}customElements.define("settings-form",y5);class E5 extends U{static properties={settings:{state:!0},error:{state:!0},notice:{state:!0},loading:{state:!0},saving:{state:!0}};constructor(){super();this.settings=null,this.error="",this.notice="",this.loading=!1,this.saving=!1}static styles=H`
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
  `;connectedCallback(){super.connectedCallback(),this.load()}async load(){this.loading=!0;try{let Z=await O.getSettings();this.settings=Z.settings??null,this.error=Z.error??""}catch(Z){this.error=Z.message}finally{this.loading=!1}}onSave=async(Z)=>{this.saving=!0,this.notice="",this.error="";try{this.settings=await O.saveSettings(Z.detail),this.notice="Settings saved."}catch(K){this.error=K.message}finally{this.saving=!1,await this.load()}};render(){return G`
      <div class="panel">
        <h2>ECU settings</h2>
        ${this.notice?G`<div class="banner ok">${this.notice}</div>`:j}
        ${this.error?G`<div class="banner err">⚠ ${this.error}</div>`:j}
        ${this.loading&&!this.settings?G`<div class="loading">Loading…</div>`:G`<settings-form
              .settings=${this.settings??{ecu_id:"",mac:"",pan_override:"",zigbee_type:"apsystems"}}
              @save=${this.onSave}
            ></settings-form>`}
      </div>
    `}}customElements.define("settings-view",E5);class x5 extends U{static properties={items:{attribute:!1},route:{type:String},open:{type:Boolean}};constructor(){super();this.items=[],this.route="dashboard",this.open=!1}close=()=>{this.dispatchEvent(new CustomEvent("close",{bubbles:!0,composed:!0}))};static styles=H`
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
      ${this.open?G`<div class="scrim" @click=${this.close}></div>`:j}
    `}}customElements.define("app-nav",x5);var xZ=[{id:"dashboard",label:"Dashboard",icon:"▮▮"},{id:"inverters",label:"Inverters",icon:"⌁"},{id:"alarms",label:"Alarms",icon:"!"},{id:"events",label:"Events",icon:"≣"},{id:"profiles",label:"Profiles",icon:"⛭"},{id:"settings",label:"Settings",icon:"⚙"}];class b5 extends U{static properties={ready:{state:!0},authed:{state:!0},configured:{state:!0},route:{state:!0},fleet:{state:!0},system:{state:!0},names:{state:!0},customProfiles:{state:!0},navOpen:{state:!0}};closeSSE=null;sysTimer=null;settingsCache=null;constructor(){super();this.ready=!1,this.authed=!1,this.configured=!0,this.route="dashboard",this.fleet=null,this.system=null,this.names={},this.customProfiles={},this.navOpen=!1}static styles=H`
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
  `;connectedCallback(){super.connectedCallback(),window.addEventListener("hashchange",this.onHash),this.onHash(),this.init()}disconnectedCallback(){super.disconnectedCallback(),window.removeEventListener("hashchange",this.onHash),this.stopStreams()}onHash=()=>{let Z=(location.hash.replace(/^#\/?/,"")||"dashboard").split("/")[0];if(this.route=xZ.some((K)=>K.id===Z)?Z:"dashboard",this.navOpen=!1,this.route==="dashboard"&&this.authed)this.fetchOverlays()};async init(){try{let Z=await O.authStatus();if(this.configured=Z.configured,this.authed=Z.authenticated,this.authed)this.startStreams()}catch{}finally{this.ready=!0}}onAuthed=async()=>{this.authed=!0,this.startStreams()};logout=async()=>{try{await O.logout()}catch{}this.authed=!1,this.stopStreams(),this.fleet=null,this.system=null};startStreams(){this.stopStreams(),this.closeSSE=Y5((K)=>{this.fleet=K});let Z=()=>O.system().then((K)=>this.system=K).catch(()=>{});Z(),this.sysTimer=setInterval(Z,5000),this.fetchSettings(),this.fetchOverlays()}async fetchSettings(){try{let Z=await O.getSettings();if(Z.settings)this.settingsCache=Z.settings,this.names=Z.settings.inverter_names??{}}catch{}}async fetchOverlays(){try{let Z=await O.overlays(),K={};for(let Q of Z)for(let $ of Q.uids)K[$]=Q.id;this.customProfiles=K}catch{}}onRename=async(Z)=>{let{uid:K,name:Q}=Z.detail,$=this.settingsCache??{ecu_id:"",mac:"",pan_override:"",zigbee_type:""},B={...$.inverter_names??{}};if(Q.trim())B[K]=Q.trim();else delete B[K];let Y={...$,inverter_names:B};try{await O.saveSettings(Y),this.settingsCache=Y,this.names=B}catch{}};stopStreams(){if(this.closeSSE?.(),this.closeSSE=null,this.sysTimer)clearInterval(this.sysTimer);this.sysTimer=null}activeView(){switch(this.route){case"inverters":return G`<inverters-view
          .fleet=${this.fleet}
          .names=${this.names}
          @rename=${this.onRename}
        ></inverters-view>`;case"alarms":return G`<alarms-view .fleet=${this.fleet}></alarms-view>`;case"events":return G`<events-view></events-view>`;case"profiles":return G`<profiles-view></profiles-view>`;case"settings":return G`<settings-view></settings-view>`;default:return G`<dashboard-view
          .fleet=${this.fleet}
          .system=${this.system}
          .names=${this.names}
          .profiles=${this.customProfiles}
        ></dashboard-view>`}}render(){if(!this.ready)return j;if(!this.authed)return G`<login-view .configured=${this.configured} @authed=${this.onAuthed}></login-view>`;let Z=xZ.find((Q)=>Q.id===this.route)?.label??"Dashboard",K=this.system?.invdriver_connected??!1;return G`
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
    `}}customElements.define("ecu-app",b5);
