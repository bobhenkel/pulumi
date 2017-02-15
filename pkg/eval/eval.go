// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"math"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/binder"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Interpreter can evaluate compiled MuPackages.
type Interpreter interface {
	core.Phase

	Ctx() *binder.Context // the binding context object.

	// EvaluatePackage performs evaluation on the given blueprint package.
	EvaluatePackage(pkg *symbols.Package, args core.Args)
	// EvaluateModule performs evaluation on the given module's entrypoint function.
	EvaluateModule(mod *symbols.Module, args core.Args)
	// EvaluateFunction performs an evaluation of the given function, using the provided arguments.
	EvaluateFunction(fnc symbols.Function, this *rt.Object, args core.Args)
}

// InterpreterHooks is a set of callbacks that can be used to hook into interesting interpreter events.
type InterpreterHooks interface {
	OnNewObject(o *rt.Object)                                            // invoked when an object is created.
	OnVariableAssign(o *rt.Object, name tokens.Name, old, nw *rt.Object) // invoked when a property is (re)assigned.
	OnEnterPackage(pkg *symbols.Package)                                 // invoked when we enter a new package.
	OnLeavePackage(pkg *symbols.Package)                                 // invoked when we leave a package.
	OnEnterModule(mod *symbols.Module)                                   // invoked when we enter a new module.
	OnLeaveModule(mod *symbols.Module)                                   // invoked when we leave a module.
	OnEnterFunction(fnc symbols.Function)                                // invoked when we enter a new function.
	OnLeaveFunction(fnc symbols.Function)                                // invoked when we leave a function.
}

// New creates an interpreter that can be used to evaluate MuPackages.
func New(ctx *binder.Context, hooks InterpreterHooks) Interpreter {
	e := &evaluator{
		ctx:        ctx,
		hooks:      hooks,
		alloc:      NewAllocator(hooks),
		globals:    make(globalMap),
		statics:    make(staticMap),
		modinits:   make(modinitMap),
		classinits: make(classinitMap),
	}
	newLocalScope(&e.locals, true, ctx.Scope)
	contract.Assert(e.locals != nil)
	return e
}

type evaluator struct {
	fnc        symbols.Function // the function currently under evaluation.
	ctx        *binder.Context  // the binding context with type and symbol information.
	hooks      InterpreterHooks // callbacks that hook into interpreter events.
	alloc      *Allocator       // the object allocator.
	globals    globalMap        // the object values for global variable symbols.
	statics    staticMap        // the object values for all static variable symbols.
	protos     prototypeMap     // the current "prototypes" for all classes.
	stack      *rt.StackFrame   // a stack of frames to keep track of calls.
	locals     *localScope      // local variable values scoped by the lexical structure.
	modinits   modinitMap       // a map of which modules have been initialized already.
	classinits classinitMap     // a map of which classes have been initialized already.
}

type globalMap map[*symbols.Module]rt.PropertyMap
type staticMap map[*symbols.Class]rt.PropertyMap
type prototypeMap map[symbols.Type]*rt.Object
type modinitMap map[*symbols.Module]bool
type classinitMap map[*symbols.Class]bool

var _ Interpreter = (*evaluator)(nil)

func (e *evaluator) Ctx() *binder.Context { return e.ctx }
func (e *evaluator) Diag() diag.Sink      { return e.ctx.Diag }

// EvaluatePackage performs evaluation on the given blueprint package.
func (e *evaluator) EvaluatePackage(pkg *symbols.Package, args core.Args) {
	glog.Infof("Evaluating package '%v'", pkg.Name())
	if e.hooks != nil {
		e.hooks.OnEnterPackage(pkg)
	}

	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of package '%v' completed w/ %v warnings and %v errors",
			pkg.Name(), e.Diag().Warnings(), e.Diag().Errors())
		if e.hooks != nil {
			e.hooks.OnLeavePackage(pkg)
		}
	}

	// Search the package for a default module "index" to evaluate.
	defmod := pkg.Default()
	if defmod == nil {
		e.Diag().Errorf(errors.ErrorPackageHasNoDefaultModule.At(pkg.Tree()), pkg.Name())
	} else {
		mod := pkg.Modules[*defmod]
		contract.Assert(mod != nil)
		e.EvaluateModule(mod, args)
	}
}

// EvaluateModule performs evaluation on the given module's entrypoint function.
func (e *evaluator) EvaluateModule(mod *symbols.Module, args core.Args) {
	glog.Infof("Evaluating module '%v'", mod.Token())
	if e.hooks != nil {
		e.hooks.OnEnterModule(mod)
	}

	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of module '%v' completed w/ %v warnings and %v errors",
			mod.Token(), e.Diag().Warnings(), e.Diag().Errors())
		if e.hooks != nil {
			e.hooks.OnLeaveModule(mod)
		}
	}

	// Fetch the module's entrypoint function, erroring out if it doesn't have one.
	hadEntry := false
	if entry, has := mod.Members[tokens.EntryPointFunction]; has {
		if entryfnc, ok := entry.(symbols.Function); ok {
			e.EvaluateFunction(entryfnc, nil, args)
			hadEntry = true
		}
	}

	if !hadEntry {
		e.Diag().Errorf(errors.ErrorModuleHasNoEntryPoint.At(mod.Tree()), mod.Name())
	}
}

// EvaluateFunction performs an evaluation of the given function, using the provided arguments, returning its graph.
func (e *evaluator) EvaluateFunction(fnc symbols.Function, this *rt.Object, args core.Args) {
	glog.Infof("Evaluating function '%v'", fnc.Token())
	if e.hooks != nil {
		e.hooks.OnEnterFunction(fnc)
	}

	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of function '%v' completed w/ %v warnings and %v errors",
			fnc.Token(), e.Diag().Warnings(), e.Diag().Errors())
		if e.hooks != nil {
			e.hooks.OnLeaveFunction(fnc)
		}
	}

	// Ensure that initializers have been run.
	switch f := fnc.(type) {
	case *symbols.ClassMethod:
		e.ensureClassInit(f.Parent)
	case *symbols.ModuleMethod:
		e.ensureModuleInit(f.Parent)
	default:
		contract.Failf("Unrecognized function evaluation type: %v", reflect.TypeOf(f))
	}

	// First, validate any arguments, and turn them into real runtime *rt.Objects.
	var argos []*rt.Object
	params := fnc.FuncNode().GetParameters()
	if params == nil {
		if len(args) != 0 {
			e.Diag().Errorf(errors.ErrorFunctionArgMismatch.At(fnc.Tree()), 0, len(args))
		}
	} else {
		if len(*params) != len(args) {
			e.Diag().Errorf(errors.ErrorFunctionArgMismatch.At(fnc.Tree()), 0, len(args))
		}

		ptys := fnc.FuncType().Parameters
		found := make(map[tokens.Name]bool)
		for i, param := range *params {
			pname := param.Name.Ident
			if arg, has := args[pname]; has {
				found[pname] = true
				argo := e.alloc.NewConstant(arg)
				if !types.CanConvert(argo.Type(), ptys[i]) {
					e.Diag().Errorf(errors.ErrorFunctionArgIncorrectType.At(fnc.Tree()), ptys[i], argo.Type())
					break
				}
				argos = append(argos, argo)
			} else {
				e.Diag().Errorf(errors.ErrorFunctionArgNotFound.At(fnc.Tree()), param.Name)
			}
		}
		for arg := range args {
			if !found[arg] {
				e.Diag().Errorf(errors.ErrorFunctionArgUnknown.At(fnc.Tree()), arg)
			}
		}
	}

	if e.Diag().Success() {
		// If the arguments bound correctly, make the call.
		_, uw := e.evalCall(fnc.Tree(), fnc, this, argos...)
		if uw != nil {
			// If the call had an unwind out of it, then presumably we have an unhandled exception.
			e.issueUnhandledException(uw, errors.ErrorUnhandledException.At(fnc.Tree()))
		}
	}

	// Dump the evaluation state at log-level 5, if it is enabled.
	e.dumpEvalState(5)
}

// Utility functions

// dumpEvalState logs the evaluator's current state at the given log-level.
func (e *evaluator) dumpEvalState(v glog.Level) {
	if glog.V(v) {
		glog.V(v).Infof("Evaluator state dump:")
		glog.V(v).Infof("=====================")

		// Print all initialized modules in alphabetical order.
		modtoks := make([]string, 0, len(e.modinits))
		for mod := range e.modinits {
			modtoks = append(modtoks, string(mod.Token()))
		}
		sort.Strings(modtoks)
		for _, mod := range modtoks {
			glog.V(v).Infof("Module init: %v", mod)
		}

		// Print all initialized classes in alphabetical order.
		classtoks := make([]string, 0, len(e.classinits))
		for class := range e.classinits {
			classtoks = append(classtoks, string(class.Token()))
		}
		sort.Strings(classtoks)
		for _, class := range classtoks {
			glog.V(v).Infof("Class init: %v", class)
		}
	}
}

// initProperty initializes a property entry in the given map, using an optional `this` pointer for member functions.
// It returns the resulting pointer along with a boolean to indicate whether the property was left unfrozen.
func (e *evaluator) initProperty(properties rt.PropertyMap, sym symbols.Symbol, this *rt.Object) (*rt.Pointer, bool) {
	k := rt.PropertyKey(sym.Name())
	switch m := sym.(type) {
	case symbols.Function:
		// A function results in a closure object referring to `this`, if any.
		// TODO[marapongo/mu#56]: all methods are readonly; consider permitting JS-style overwriting of them.
		obj := e.alloc.NewFunction(m, this)
		if this != nil && e.hooks != nil {
			e.hooks.OnVariableAssign(this, tokens.Name(k), nil, obj)
		}
		fnc := e.alloc.NewFunction(m, this)
		return properties.InitAddr(k, fnc, true), false
	case symbols.Variable:
		// A variable could have a default object; if so, use that; otherwise, null will be substituted automatically.
		var obj *rt.Object
		if m.Default() != nil {
			obj = e.alloc.NewConstant(*m.Default())
			if this != nil && e.hooks != nil {
				e.hooks.OnVariableAssign(this, tokens.Name(k), nil, obj)
			}
		}
		ptr := properties.InitAddr(k, obj, false)
		return ptr, m.Readonly()
	case *symbols.Class:
		// A class resolves to its prototype object.
		proto := e.getPrototype(m)
		return properties.InitAddr(k, proto, false), false
	default:
		contract.Failf("Unrecognized property '%v' symbol type: %v", k, reflect.TypeOf(sym))
		return nil, false
	}
}

// ensureClassInit ensures that the target's class initializer has been run.
func (e *evaluator) ensureClassInit(class *symbols.Class) {
	already := e.classinits[class]
	e.classinits[class] = true // set true before running, in case of cycles.

	if !already {
		// First ensure the module initializer has run.
		e.ensureModuleInit(class.Parent)

		// Now populate this class's statics with all of the static members.
		var readonlines []*rt.Pointer
		statics := e.getClassStatics(class)
		var current symbols.Type = class
		for current != nil {
			members := current.TypeMembers()
			for _, member := range symbols.StableClassMemberMap(members) {
				if m := members[member]; m.Static() {
					if ptr, readonly := e.initProperty(statics, m, nil); readonly {
						// Readonly properties are unfrozen during initialization; afterwards, they will be frozen.
						readonlines = append(readonlines, ptr)
					}
				}
			}

			// Keep going up the type hierarchy.
			current = current.Base()
		}

		// Next, run the class if it has an initializer.
		if init := class.GetInit(); init != nil {
			glog.V(7).Infof("Initializing class: %v", class)
			contract.Assert(len(init.Ty.Parameters) == 0)
			contract.Assert(init.Ty.Return == nil)
			ret, uw := e.evalCall(class.Tree(), init, nil)
			contract.Assert(ret == nil)
			if uw != nil {
				// Must be an unhandled exception; spew it as an error (but keep going).
				e.issueUnhandledException(uw, errors.ErrorUnhandledInitException.At(init.Tree()), class)
			}
		} else {
			glog.V(7).Infof("Class has no initializer: %v", class)
		}

		// Now, finally, ensure that all readonly class statics are frozen.
		for _, readonly := range readonlines {
			readonly.Freeze() // ensure this cannot be written post-initialization.
		}
	}
}

// ensureModuleInit ensures that the target's module initializer has been run.  It also evaluates dependency module
// initializers, assuming they have been declared.  If they have not, those will run when we access them.
func (e *evaluator) ensureModuleInit(mod *symbols.Module) {
	already := e.modinits[mod]
	e.modinits[mod] = true // set true before running, in case of cycles.

	if !already {
		// First ensure all imported module initializers are run, in the order in which they were given.
		for _, imp := range mod.Imports {
			e.ensureModuleInit(imp)
		}

		// Populate all properties in this module, even if they will be empty for now.
		var readonlines []*rt.Pointer
		globals := e.getModuleGlobals(mod)
		for _, member := range symbols.StableModuleMemberMap(mod.Members) {
			if ptr, readonly := e.initProperty(globals, mod.Members[member], nil); readonly {
				// If this property was left unfrozen, be sure to remember it for freezing after we're done.
				readonlines = append(readonlines, ptr)
			}
		}

		// Next, run the module initializer if it has one.
		if init := mod.GetInit(); init != nil {
			glog.V(7).Infof("Initializing module: %v", mod)
			contract.Assert(len(init.Type.Parameters) == 0)
			contract.Assert(init.Type.Return == nil)
			ret, uw := e.evalCall(mod.Tree(), init, nil)
			contract.Assert(ret == nil)
			if uw != nil {
				// Must be an unhandled exception; spew it as an error (but keep going).
				e.issueUnhandledException(uw, errors.ErrorUnhandledInitException.At(init.Tree()), mod)
			}
		} else {
			glog.V(7).Infof("Module has no initializer: %v", mod)
		}

		// Ensure that all readonly module properties are frozen.
		for _, readonly := range readonlines {
			readonly.Freeze() // ensure this is never written to after initialization.
		}
	}
}

// getModuleGlobals returns a module's globals, lazily initializing if needed.
func (e *evaluator) getModuleGlobals(module *symbols.Module) rt.PropertyMap {
	globals, has := e.globals[module]
	if !has {
		globals = make(rt.PropertyMap)
		e.globals[module] = globals
	}
	return globals
}

// getClassStatics returns a statics table, lazily initializing if needed.
func (e *evaluator) getClassStatics(class *symbols.Class) rt.PropertyMap {
	statics, has := e.statics[class]
	if !has {
		statics = make(rt.PropertyMap)
		e.statics[class] = statics
	}
	return statics
}

// getPrototype returns the prototype for a given type.  The prototype is a mutable object, and so it is cached, and
// reused for subsequent lookups.  This means that mutations in the prototype are lasting and visible for all later
// uses.  This is similar to ECMAScript behavior; see http://www.ecma-international.org/ecma-262/6.0/#sec-objects.
// TODO[marapongo/mu#70]: technically this should be gotten from the constructor function object; we will need to
//     rewire things a bit, depending on how serious we are about ECMAScript compliance, especially dynamic scenarios.
func (e *evaluator) getPrototype(t symbols.Type) *rt.Object {
	// If there is already a proto for this type, use it.
	if proto, has := e.protos[t]; has {
		return proto
	}

	// If not, we need to create a new one.  First, fetch the base if there is one.
	var base *rt.Object
	if t.Base() != nil {
		base = e.getPrototype(t.Base())
	}

	// Now populate the prototype object with all members.
	members := t.TypeMembers()
	proto := e.alloc.New(symbols.NewPrototypeType(t), nil, base)
	properties := proto.Properties()
	for _, member := range symbols.StableClassMemberMap(members) {
		if m := members[member]; !m.Static() {
			e.initProperty(properties, m, proto)
		}
	}

	return proto
}

// newObject allocates a fresh object of the given type, wired up to its prototype.
func (e *evaluator) newObject(t symbols.Type) *rt.Object {
	// First, fetch the prototype chain for this object.  This is required to implement property chaining.
	proto := e.getPrototype(t)

	// Now create an empty object of the desired type.  Subsequent operations will do the right thing with it.  E.g.,
	// overwriting a property will add a new entry to the object's map; reading will search the prototpe chain; etc.
	return e.alloc.New(t, nil, proto)
}

// issueUnhandledException issues an unhandled exception error using the given diagnostic and unwind information.
func (e *evaluator) issueUnhandledException(uw *Unwind, err *diag.Diag, args ...interface{}) {
	contract.Assert(uw.Throw())

	// Produce a message with the exception text plus stack trace.
	var msg string
	if thrown := uw.Thrown(); thrown != nil {
		info := thrown.ExceptionValue()
		msg = info.Message
		msg += "\n" + info.Stack.Trace(e.Diag(), "\t", info.Node)
	} else {
		msg = "no details available"
	}

	// Now simply output the error with the message plus stack trace.
	args = append(args, msg)
	e.Diag().Errorf(err, args...)
}

// pushScope pushes a new local and context scope.  The frame argument indicates whether this is an activation frame,
// meaning that searches for local variables will not probe into parent scopes (since they are inaccessible).
func (e *evaluator) pushScope(frame *rt.StackFrame) {
	if frame != nil {
		frame.Parent = e.stack // remember the parent so we can pop.
		e.stack = frame        // install this as the current frame.
	}
	e.locals.Push(frame != nil) // pushing the local scope also updates the context scope.
}

// popScope pops the current local and context scopes.
func (e *evaluator) popScope(frame bool) {
	if frame {
		contract.Assert(e.stack != nil)
		e.stack = e.stack.Parent
	}
	e.locals.Pop() // popping the local scope also updates the context scope.
}

// Functions

func (e *evaluator) evalCall(node diag.Diagable, fnc symbols.Function,
	this *rt.Object, args ...*rt.Object) (*rt.Object, *Unwind) {
	glog.V(7).Infof("Evaluating call to fnc %v; this=%v args=%v", fnc, this != nil, len(args))

	// First check the this pointer, since it might throw before the call even happens.
	var thisVariable *symbols.LocalVariable
	var superVariable *symbols.LocalVariable
	switch f := fnc.(type) {
	case *symbols.ClassMethod:
		if f.Static() {
			contract.Assert(this == nil)
		} else if uw := e.checkThis(node, this); uw != nil {
			return nil, uw
		} else {
			thisVariable = f.Parent.This
			superVariable = f.Parent.Super
		}
	default:
		contract.Assert(this == nil)
	}

	// Ensure that we enter the right module/class context, otherwise module-sensitive binding won't work.
	switch f := fnc.(type) {
	case *symbols.ClassMethod:
		popm := e.ctx.PushModule(f.Parent.Parent)
		defer popm()
		popc := e.ctx.PushClass(f.Parent)
		defer popc()
	case *symbols.ModuleMethod:
		popm := e.ctx.PushModule(f.Parent)
		defer popm()
	default:
		contract.Failf("Unrecognized function type during call: %v", reflect.TypeOf(fnc))
	}

	// Save the prior func, set the new one, and restore upon exit.
	prior := fnc
	e.fnc = fnc
	defer func() { e.fnc = prior }()

	// Set up a new lexical scope "activation frame" in which we can bind the parameters; restore it upon exit.
	e.pushScope(&rt.StackFrame{Func: fnc, Caller: node})
	defer e.popScope(true)

	// Invoke the hooks if available.
	if e.hooks != nil {
		e.hooks.OnEnterFunction(fnc)
		defer e.hooks.OnLeaveFunction(fnc)
	}

	// If the target is an instance method, the "this" and "super" variables must be bound to values.
	if thisVariable != nil {
		contract.Assert(this != nil)
		e.ctx.Scope.Register(thisVariable)
		e.locals.InitValueAddr(thisVariable, rt.NewPointer(this, true))
	}
	if superVariable != nil {
		contract.Assert(this != nil)
		e.ctx.Scope.Register(superVariable)
		e.locals.InitValueAddr(superVariable, rt.NewPointer(this, true))
	}

	// Ensure that the arguments line up to the parameter slots and add them to the frame.
	fnode := fnc.FuncNode()
	params := fnode.GetParameters()
	if params == nil {
		contract.Assert(len(args) == 0)
	} else {
		contract.Assert(len(args) == len(*params))
		for i, param := range *params {
			sym := e.ctx.RequireVariable(param).(*symbols.LocalVariable)
			e.ctx.Scope.Register(sym)
			arg := args[i]
			contract.Assert(types.CanConvert(arg.Type(), sym.Type()))
			e.locals.SetValue(sym, arg)
		}
	}

	// Now perform the invocation by visiting the body.
	uw := e.evalBlock(fnode.GetBody())

	// Check that the unwind is as expected.  In particular:
	//     1) no breaks or continues are expected;
	//     2) any throw is treated as an unhandled exception that propagates to the caller.
	//     3) any return is checked to be of the expected type, and returned as the result of the call.
	retty := fnc.FuncType().Return
	if uw != nil {
		if uw.Throw() {
			if glog.V(7) {
				glog.V(7).Infof("Evaluated call to fnc %v; unhandled exception: %v", uw.Thrown())
			}
			return nil, uw
		}

		contract.Assert(uw.Return()) // break/continue not expected.
		ret := uw.Returned()
		contract.Assert((retty == nil) == (ret == nil))
		contract.Assert(ret == nil || types.CanConvert(ret.Type(), retty))
		if glog.V(7) {
			glog.V(7).Infof("Evaluated call to fnc %v; return=%v", fnc, ret)
		}
		return ret, nil
	}

	// An absence of a return is okay for void-returning functions.
	contract.Assert(retty == nil)
	glog.V(7).Infof("Evaluated call to fnc %v; return=<nil>", fnc)
	return nil, nil
}

// Statements

func (e *evaluator) evalStatement(node ast.Statement) *Unwind {
	if glog.V(7) {
		glog.V(7).Infof("Evaluating statement: %v", reflect.TypeOf(node))
	}

	// Simply switch on the node type and dispatch to the specific function, returning the Unwind info.
	switch n := node.(type) {
	case *ast.Block:
		return e.evalBlock(n)
	case *ast.LocalVariableDeclaration:
		return e.evalLocalVariableDeclaration(n)
	case *ast.TryCatchFinally:
		return e.evalTryCatchFinally(n)
	case *ast.BreakStatement:
		return e.evalBreakStatement(n)
	case *ast.ContinueStatement:
		return e.evalContinueStatement(n)
	case *ast.IfStatement:
		return e.evalIfStatement(n)
	case *ast.LabeledStatement:
		return e.evalLabeledStatement(n)
	case *ast.ReturnStatement:
		return e.evalReturnStatement(n)
	case *ast.ThrowStatement:
		return e.evalThrowStatement(n)
	case *ast.WhileStatement:
		return e.evalWhileStatement(n)
	case *ast.EmptyStatement:
		return nil // nothing to do
	case *ast.MultiStatement:
		return e.evalMultiStatement(n)
	case *ast.ExpressionStatement:
		return e.evalExpressionStatement(n)
	default:
		contract.Failf("Unrecognized statement node kind: %v", node.GetKind())
		return nil
	}
}

func (e *evaluator) evalBlock(node *ast.Block) *Unwind {
	// Push a scope at the start, and pop it at afterwards; both for the symbol context and local variable values.
	e.pushScope(nil)
	defer e.popScope(false)

	for _, stmt := range node.Statements {
		if uw := e.evalStatement(stmt); uw != nil {
			return uw
		}
	}

	return nil
}

func (e *evaluator) evalLocalVariableDeclaration(node *ast.LocalVariableDeclaration) *Unwind {
	// Populate the variable in the scope.
	sym := e.ctx.RequireVariable(node.Local).(*symbols.LocalVariable)
	e.ctx.Scope.Register(sym)

	// If there is a default value, set it now.
	if node.Local.Default != nil {
		obj := e.alloc.NewConstant(*node.Local.Default)
		e.locals.SetValue(sym, obj)
	}

	return nil
}

func (e *evaluator) evalTryCatchFinally(node *ast.TryCatchFinally) *Unwind {
	// First, execute the TryBlock.
	uw := e.evalBlock(node.TryBlock)
	if uw != nil && uw.Throw() {
		// The try block threw something; see if there is a handler that covers this.
		thrown := uw.Thrown()
		if node.CatchBlocks != nil {
			for _, catch := range *node.CatchBlocks {
				ex := e.ctx.RequireVariable(catch.Exception).(*symbols.LocalVariable)
				exty := ex.Type()
				contract.Assert(types.CanConvert(exty, types.Exception))
				if types.CanConvert(thrown.Type(), exty) {
					// This type matched, so this handler will catch the exception.  Set the exception variable,
					// evaluate the block, and swap the Unwind information (thereby "handling" the in-flight exception).
					e.pushScope(nil)
					e.locals.SetValue(ex, thrown)
					uw = e.evalBlock(catch.Block)
					e.popScope(false)
					break
				}
			}
		}
	}

	// No matter the Unwind instructions, be sure to invoke the FinallyBlock.
	if node.FinallyBlock != nil {
		uwf := e.evalBlock(node.FinallyBlock)

		// Any Unwind information from the finally block overrides the try Unwind that was in flight.
		if uwf != nil {
			uw = uwf
		}
	}

	return uw
}

func (e *evaluator) evalBreakStatement(node *ast.BreakStatement) *Unwind {
	var label *tokens.Name
	if node.Label != nil {
		label = &node.Label.Ident
	}
	return NewBreakUnwind(label)
}

func (e *evaluator) evalContinueStatement(node *ast.ContinueStatement) *Unwind {
	var label *tokens.Name
	if node.Label != nil {
		label = &node.Label.Ident
	}
	return NewContinueUnwind(label)
}

func (e *evaluator) evalIfStatement(node *ast.IfStatement) *Unwind {
	// Evaluate the branches explicitly based on the result of the condition node.
	cond, uw := e.evalExpression(node.Condition)
	if uw != nil {
		return uw
	}
	if cond.BoolValue() {
		return e.evalStatement(node.Consequent)
	} else if node.Alternate != nil {
		return e.evalStatement(*node.Alternate)
	}
	return nil
}

func (e *evaluator) evalLabeledStatement(node *ast.LabeledStatement) *Unwind {
	// Evaluate the underlying statement; if it is breaking or continuing to this label, stop the Unwind.
	uw := e.evalStatement(node.Statement)
	if uw != nil && uw.Label() != nil && *uw.Label() == node.Label.Ident {
		contract.Assert(uw.Continue() || uw.Break())
		// TODO: perform correct break/continue behavior when the label is affixed to a loop.
		uw = nil
	}
	return uw
}

func (e *evaluator) evalReturnStatement(node *ast.ReturnStatement) *Unwind {
	var ret *rt.Object
	if node.Expression != nil {
		var uw *Unwind
		if ret, uw = e.evalExpression(*node.Expression); uw != nil {
			// If the expression caused an Unwind, propagate that and ignore the returned object.
			return uw
		}
	}
	return NewReturnUnwind(ret)
}

func (e *evaluator) evalThrowStatement(node *ast.ThrowStatement) *Unwind {
	thrown, uw := e.evalExpression(node.Expression)
	if uw != nil {
		// If the throw expression itself threw an exception, propagate that instead.
		return uw
	}
	contract.Assert(thrown != nil)
	return NewThrowUnwind(thrown)
}

func (e *evaluator) evalWhileStatement(node *ast.WhileStatement) *Unwind {
	// So long as the test evaluates to true, keep on visiting the body.
	var uw *Unwind
	for {
		test, uw := e.evalExpression(node.Test)
		if uw != nil {
			return uw
		}
		if test.BoolValue() {
			if uws := e.evalStatement(node.Body); uw != nil {
				if uws.Continue() {
					contract.Assertf(uws.Label() == nil, "Labeled continue not yet supported")
					continue
				} else if uws.Break() {
					contract.Assertf(uws.Label() == nil, "Labeled break not yet supported")
					break
				} else {
					// If it's not a continue or break, stash the Unwind away and return it.
					uw = uws
					break
				}
			}
		} else {
			break
		}
	}
	return uw // usually nil, unless a body statement threw/returned.
}

func (e *evaluator) evalMultiStatement(node *ast.MultiStatement) *Unwind {
	for _, stmt := range node.Statements {
		if uw := e.evalStatement(stmt); uw != nil {
			return uw
		}
	}
	return nil
}

func (e *evaluator) evalExpressionStatement(node *ast.ExpressionStatement) *Unwind {
	// Just evaluate the expression, drop its object on the floor, and propagate its Unwind information.
	_, uw := e.evalExpression(node.Expression)
	return uw
}

// Expressions

func (e *evaluator) evalExpression(node ast.Expression) (*rt.Object, *Unwind) {
	if glog.V(7) {
		glog.V(7).Infof("Evaluating expression: %v", reflect.TypeOf(node))
		debug.PrintStack()
	}

	// Simply switch on the node type and dispatch to the specific function, returning the object and Unwind info.
	switch n := node.(type) {
	case *ast.NullLiteral:
		return e.evalNullLiteral(n)
	case *ast.BoolLiteral:
		return e.evalBoolLiteral(n)
	case *ast.NumberLiteral:
		return e.evalNumberLiteral(n)
	case *ast.StringLiteral:
		return e.evalStringLiteral(n)
	case *ast.ArrayLiteral:
		return e.evalArrayLiteral(n)
	case *ast.ObjectLiteral:
		return e.evalObjectLiteral(n)
	case *ast.LoadLocationExpression:
		return e.evalLoadLocationExpression(n)
	case *ast.LoadDynamicExpression:
		return e.evalLoadDynamicExpression(n)
	case *ast.NewExpression:
		return e.evalNewExpression(n)
	case *ast.InvokeFunctionExpression:
		return e.evalInvokeFunctionExpression(n)
	case *ast.LambdaExpression:
		return e.evalLambdaExpression(n)
	case *ast.UnaryOperatorExpression:
		return e.evalUnaryOperatorExpression(n)
	case *ast.BinaryOperatorExpression:
		return e.evalBinaryOperatorExpression(n)
	case *ast.CastExpression:
		return e.evalCastExpression(n)
	case *ast.IsInstExpression:
		return e.evalIsInstExpression(n)
	case *ast.TypeOfExpression:
		return e.evalTypeOfExpression(n)
	case *ast.ConditionalExpression:
		return e.evalConditionalExpression(n)
	case *ast.SequenceExpression:
		return e.evalSequenceExpression(n)
	default:
		contract.Failf("Unrecognized expression node kind: %v", node.GetKind())
		return nil, nil
	}
}

// evalLValueExpression evaluates an expression for use as an l-value; in particular, this loads the target as a
// pointer/reference object, rather than as an ordinary value, so that it can be used in an assignment.  This is only
// valid on the subset of AST nodes that are legal l-values (very few of them, it turns out).
func (e *evaluator) evalLValueExpression(node ast.Expression) (location, *Unwind) {
	switch n := node.(type) {
	case *ast.LoadLocationExpression:
		return e.evalLoadLocation(n, true)
	case *ast.LoadDynamicExpression:
		return e.evalLoadDynamic(n, true)
	case *ast.UnaryOperatorExpression:
		contract.Assert(n.Operator == ast.OpDereference)
		obj, uw := e.evalUnaryOperatorExpressionFor(n, true)
		return location{Obj: obj}, uw
	default:
		contract.Failf("Unrecognized l-value expression type: %v", node.GetKind())
		return location{}, nil
	}
}

func (e *evaluator) evalNullLiteral(node *ast.NullLiteral) (*rt.Object, *Unwind) {
	return e.alloc.NewNull(), nil
}

func (e *evaluator) evalBoolLiteral(node *ast.BoolLiteral) (*rt.Object, *Unwind) {
	return e.alloc.NewBool(node.Value), nil
}

func (e *evaluator) evalNumberLiteral(node *ast.NumberLiteral) (*rt.Object, *Unwind) {
	return e.alloc.NewNumber(node.Value), nil
}

func (e *evaluator) evalStringLiteral(node *ast.StringLiteral) (*rt.Object, *Unwind) {
	return e.alloc.NewString(node.Value), nil
}

func (e *evaluator) evalArrayLiteral(node *ast.ArrayLiteral) (*rt.Object, *Unwind) {
	// Fetch this expression type and assert that it's an array.
	ty := e.ctx.RequireType(node).(*symbols.ArrayType)

	// Now create the array data.
	var sz *int
	var arr []*rt.Pointer

	// If there's a node size, ensure it's a number, and initialize the array.
	if node.Size != nil {
		sze, uw := e.evalExpression(*node.Size)
		if uw != nil {
			return nil, uw
		}
		// TODO: this really ought to be an int, not a float...
		sz := int(sze.NumberValue())
		if sz < 0 {
			// If the size is less than zero, raise a new error.
			return nil, NewThrowUnwind(e.NewNegativeArrayLengthException(*node.Size))
		}
		arr = make([]*rt.Pointer, sz)
	}

	// Allocate a new array object.
	arrobj := e.alloc.NewArray(ty.Element, &arr)

	// If there are elements, place them into the array.  This has two behaviors:
	//     1) if there is a size, there can be up to that number of elements, which are set;
	//     2) if there is no size, all of the elements are appended to the array.
	if node.Elements != nil {
		if sz == nil {
			// Right-size the array.
			arr = make([]*rt.Pointer, 0, len(*node.Elements))
		} else if len(*node.Elements) > *sz {
			// The element count exceeds the size; raise an error.
			return nil, NewThrowUnwind(
				e.NewIncorrectArrayElementCountException(node, *sz, len(*node.Elements)))
		}

		for i, elem := range *node.Elements {
			elemobj, uw := e.evalExpression(elem)
			if uw != nil {
				return nil, uw
			}
			elemptr := rt.NewPointer(elemobj, false)
			if sz == nil {
				arr = append(arr, elemptr)
			} else {
				arr[i] = elemptr
			}

			// Track all assignments.
			if e.hooks != nil {
				e.hooks.OnVariableAssign(arrobj, tokens.Name(strconv.Itoa(i)), nil, elemobj)
			}
		}
	}

	return arrobj, nil
}

func (e *evaluator) evalObjectLiteral(node *ast.ObjectLiteral) (*rt.Object, *Unwind) {
	ty := e.ctx.Types[node]

	// Allocate a new object of the right type, containing all of the properties pre-populated.
	obj := e.newObject(ty)

	if node.Properties != nil {
		// The binder already checked that the properties are legal, so we will simply store them as values.
		for _, init := range *node.Properties {
			val, uw := e.evalExpression(init.Value)
			if uw != nil {
				return nil, uw
			}

			// For dynamic types, we simply store the values in the bag of properties.  For all other types, we actually
			// require that the token be a class member token that references a valid property.
			id := init.Property.Tok
			var addr *rt.Pointer
			var property rt.PropertyKey
			if ty == types.Dynamic {
				property = rt.PropertyKey(id)
				addr = obj.GetPropertyAddr(property, true, true)
			} else {
				contract.Assert(id.HasClassMember())
				member := tokens.ClassMember(id).Name()
				property = rt.PropertyKey(member.Name())
				addr = obj.GetPropertyAddr(property, true, true)
			}
			addr.Set(val)

			// Track all assignments.
			if e.hooks != nil {
				e.hooks.OnVariableAssign(obj, tokens.Name(property), nil, val)
			}
		}
	}

	// Ensure we freeze anything that must be frozen.
	obj.FreezeReadonlyProperties()

	return obj, nil
}

func (e *evaluator) evalLoadLocationExpression(node *ast.LoadLocationExpression) (*rt.Object, *Unwind) {
	loc, uw := e.evalLoadLocation(node, false)
	return loc.Obj, uw
}

type location struct {
	This *rt.Object  // the target object, if any.
	Name tokens.Name // the simple name of the variable.
	Lval bool        // whether the result is an lval.
	Obj  *rt.Object  // the resulting object (pointer if lval, object otherwise).
}

// getObjectOrSuperProperty loads a property pointer from an object using the given property key.  It understands how
// to determine whether this is a `super` load, and bind it, and will adjust the resulting pointer accordingly.
func (e *evaluator) getObjectOrSuperProperty(
	obj *rt.Object, objexpr ast.Expression, k rt.PropertyKey, init bool, forWrite bool) *rt.Pointer {
	// If this member is being accessed using "super", we need to start our property search from the
	// superclass prototype, and not the object itself, so that we find the right value.
	super := false
	if objexpr != nil {
		if ldloc, isldloc := objexpr.(*ast.LoadLocationExpression); isldloc {
			if ldloc.Name.Tok == tokens.Token(tokens.SuperVariable) {
				contract.Assert(ldloc.Object == nil)
				super = true
			}
		}
	}

	// If a superclass, use the right prototype.
	var target *rt.Object
	if super {
		proto := obj.Proto()
		contract.Assertf(proto != nil, "Expected a prototype for a class object involved in `super`")
		target = proto.Proto()
		contract.Assertf(target != nil, "Expected a superclass prototype when accessing `super`")
		// TODO: adjust the this object.
	} else {
		// Otherwise, simply fetch the property from the object directly.
		target = obj
	}

	return target.GetPropertyAddr(k, init, forWrite)
}

// evalLoadLocation evaluates and loads information about the target.  It takes an lval bool which
// determines whether the resulting location object is an lval (pointer) or regular object.
func (e *evaluator) evalLoadLocation(node *ast.LoadLocationExpression, lval bool) (location, *Unwind) {
	// If there's a target object, evaluate it.
	var this *rt.Object
	var thisexpr ast.Expression
	if node.Object != nil {
		thisexpr = *node.Object
		var uw *Unwind
		if this, uw = e.evalExpression(thisexpr); uw != nil {
			return location{}, uw
		}
	}

	// Create a pointer to the target location.
	var pv *rt.Pointer
	var ty symbols.Type
	var sym symbols.Symbol
	tok := node.Name.Tok
	if tok.Simple() {
		// If there is no object and the name is simple, it refers to a local variable in the current scope.
		contract.Assert(this == nil)
		loc := e.ctx.Scope.Lookup(tok.Name())
		contract.Assert(loc != nil)
		pv = e.locals.GetValueAddr(loc, true)
		ty = loc.Type()
		sym = loc
	} else {
		sym = e.ctx.LookupSymbol(node.Name, tok, false)
		contract.Assert(sym != nil) // don't issue errors; we shouldn't ever get here if verification failed.

		// If the symbol is an export, keep chasing down the referents until we hit a real symbol.
		for {
			export, isexport := sym.(*symbols.Export)
			if !isexport {
				break
			}
			// Simply chase the referent symbol until we bottom out on something useful.
			contract.Assertf(export.Referent != sym, "Unexpected self-referential export token")
			sym = export.Referent
			contract.Assertf(sym != nil, "Expected export '%v' to resolve to a token", export.Node.Referent.Tok)
		}

		// Look up the symbol property in the right place.  Note that because this is a static load, we
		// intentionally do not perform any lazily initialization of missing property slots; they must exist.
		switch s := sym.(type) {
		case symbols.ClassMember:
			// Consult either the statics map or the object's property based on the kind of symbol.  Note that we do
			// this even for class functions so that in case they are replaced or overridden in derived types, we get
			// the expected "virtual" dispatch behavior.  The one special case is constructors, where we intentionally
			// return a statically resolved symbol (since they aren't stored as properties and to support `super`).
			k := rt.PropertyKey(sym.Name())
			if s.Static() {
				contract.Assert(this == nil)
				statics := e.getClassStatics(s.MemberParent())
				pv = statics.GetAddr(k, false)
			} else {
				contract.Assert(this != nil)
				if uw := e.checkThis(node, this); uw != nil {
					return location{}, uw
				}
				pv = e.getObjectOrSuperProperty(this, thisexpr, k, false, lval)
			}
			ty = s.Type()
			contract.Assert(pv != nil)
			contract.Assert(ty != nil)
		case symbols.ModuleMemberProperty:
			// Search the globals table for this module.
			contract.Assert(this == nil)
			k := rt.PropertyKey(s.Name())
			globals := e.getModuleGlobals(s.MemberParent())
			pv = globals.GetAddr(k, false)
			ty = s.MemberType()
			contract.Assert(pv != nil)
			contract.Assert(ty != nil)
		default:
			contract.Failf("Unexpected symbol token kind during load expression: %v", tok)
		}
	}

	// If this is an l-value, return a pointer to the object; otherwise, return the raw object.
	var obj *rt.Object
	if lval {
		obj = e.alloc.NewPointer(ty, pv)
	} else {
		obj = pv.Obj()
	}

	return location{
		This: this,
		Name: sym.Name(),
		Lval: lval,
		Obj:  obj,
	}, nil
}

// checkThis checks a this object, raising a runtime error if it is the runtime null value.
func (e *evaluator) checkThis(node diag.Diagable, this *rt.Object) *Unwind {
	contract.Assert(this != nil) // binder should catch cases where this isn't true
	if this.Type() == types.Null {
		return NewThrowUnwind(e.NewNullObjectException(node))
	}
	return nil
}

func (e *evaluator) evalLoadDynamicExpression(node *ast.LoadDynamicExpression) (*rt.Object, *Unwind) {
	loc, uw := e.evalLoadDynamic(node, false)
	return loc.Obj, uw
}

func (e *evaluator) evalLoadDynamic(node *ast.LoadDynamicExpression, lval bool) (location, *Unwind) {
	var uw *Unwind

	// Evaluate the object and then the property expression.
	var this *rt.Object
	if this, uw = e.evalExpression(node.Object); uw != nil {
		return location{}, uw
	}
	var name *rt.Object
	if name, uw = e.evalExpression(node.Name); uw != nil {
		return location{}, uw
	}

	// Check that the object isn't null; if it is, raise an exception.
	if uw = e.checkThis(node, this); uw != nil {
		return location{}, uw
	}

	// Now go ahead and search the object for a property with the given name.
	var pv *rt.Pointer
	var key tokens.Name
	if name.Type() == types.Number {
		_, isarr := this.Type().(*symbols.ArrayType)
		contract.Assertf(isarr, "Expected an array for numeric dynamic load index")
		ix := int(name.NumberValue())
		arrv := this.ArrayValue()
		// TODO[marapongo/mu#70]: Although storing arrays as arrays is fine for many circumstances, there are two cases
		//     particular that could cause us troubles with ECMAScript compliance.  First, negative indices are fine in
		//     ECMAScript.  Second, sparse arrays can be represented more efficiently as a "bag of properties" than as a
		//     true array that needs to be resized (possibly growing to become enormous in memory usage).
		// TODO[marapongo/mu#70]: We are emulating "ECMAScript-like" array accesses, where -- just like ordinary
		//     property accesses below -- we will permit indexes that we've never seen before.  Out of bounds should
		//     yield `undefined`, rather than the usual case of throwing an exception, for example.  And such
		//     assignments are to be permitted.  This will cause troubles down the road when we do other languages that
		//     reject out of bounds accesses e.g. Python.  An alternative approach would be to require ECMAScript to
		//     use a runtime library anytime an array is accessed, translating exceptions like this into `undefined`s.
		if ix > len(*arrv) {
			newarr := make([]*rt.Pointer, ix+1)
			copy(*arrv, newarr)
			*arrv = newarr
		}
		pv = (*arrv)[ix]
		if pv == nil {
			pv = rt.NewPointer(e.alloc.NewNull(), false)
			(*arrv)[ix] = pv
		}
	} else {
		contract.Assertf(name.Type() == types.String, "Expected dynamic load name to be a string")
		key = tokens.Name(name.StringValue())
		pv = e.getObjectOrSuperProperty(this, node.Object, rt.PropertyKey(key), false, lval)
	}

	// If this isn't for an l-value, return the raw object.  Otherwise, make sure it's not readonly, and return it.
	var obj *rt.Object
	if lval {
		// A readonly reference cannot be used as an l-value.
		if pv.Readonly() {
			e.Diag().Errorf(errors.ErrorIllegalReadonlyLValue.At(node))
		}
		obj = e.alloc.NewPointer(types.Dynamic, pv)
	} else {
		obj = pv.Obj()
	}
	contract.Assert(obj != nil)

	return location{
		This: this,
		Name: tokens.Name(key),
		Lval: lval,
		Obj:  obj,
	}, nil
}

func (e *evaluator) evalNewExpression(node *ast.NewExpression) (*rt.Object, *Unwind) {
	// Fetch the type of this expression; that's the kind of object we are allocating.
	ty := e.ctx.RequireType(node)

	// Create a object of the right type, containing all of the properties pre-populated.
	obj := e.newObject(ty)

	// See if there is a constructor method.  If there isn't, we just return the fresh object.
	if ctor, has := ty.TypeMembers()[tokens.ClassConstructorFunction]; has {
		ctormeth, isfunc := ctor.(*symbols.ClassMethod)
		contract.Assertf(isfunc,
			"Expected ctor %v to be a class method; got %v", ctor, reflect.TypeOf(ctor))
		contract.Assertf(ctormeth.Ty.Return == nil,
			"Expected ctor %v to have a nil return; got %v", ctor, ctormeth.Ty.Return)

		// Evaluate the arguments in order.
		var args []*rt.Object
		if node.Arguments != nil {
			for _, arg := range *node.Arguments {
				argobj, uw := e.evalExpression(arg)
				if uw != nil {
					return nil, uw
				}
				args = append(args, argobj)
			}
		}

		// Now dispatch the function call using the fresh object as the constructor's `this` argument.
		if _, uw := e.evalCall(node, ctormeth, obj, args...); uw != nil {
			return nil, uw
		}
	} else {
		contract.Assertf(node.Arguments == nil || len(*node.Arguments) == 0,
			"No constructor found for %v, yet the new expression had %v args", ty, len(*node.Arguments))
		class, isclass := ty.(*symbols.Class)
		contract.Assertf(!isclass || class.Extends == nil,
			"No constructor found for %v, yet there is a base class; chaining must be done manually", ty)
	}

	// Finally, ensure that all readonly properties are frozen now.
	obj.FreezeReadonlyProperties()

	return obj, nil
}

func (e *evaluator) evalInvokeFunctionExpression(node *ast.InvokeFunctionExpression) (*rt.Object, *Unwind) {
	// Evaluate the function that we are meant to invoke.
	fncobj, uw := e.evalExpression(node.Function)
	if uw != nil {
		return nil, uw
	}

	// Ensure that this actually led to a function; this is guaranteed by the binder.
	var fnc rt.FuncStub
	switch fncobj.Type().(type) {
	case *symbols.FunctionType:
		fnc = fncobj.FunctionValue()
		contract.Assert(fnc.Func != nil)
	default:
		contract.Failf("Expected function expression to yield a function type")
	}

	// Now evaluate the arguments to the function, in order.
	var args []*rt.Object
	if node.Arguments != nil {
		for _, arg := range *node.Arguments {
			argobj, uw := e.evalExpression(arg)
			if uw != nil {
				return nil, uw
			}
			args = append(args, argobj)
		}
	}

	// Finally, actually dispatch the call; this will create the activation frame, etc. for us.
	return e.evalCall(node, fnc.Func, fnc.This, args...)
}

func (e *evaluator) evalLambdaExpression(node *ast.LambdaExpression) (*rt.Object, *Unwind) {
	// TODO: create the lambda object that can be invoked at runtime.
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalUnaryOperatorExpression(node *ast.UnaryOperatorExpression) (*rt.Object, *Unwind) {
	return e.evalUnaryOperatorExpressionFor(node, false)
}

func (e *evaluator) evalUnaryOperatorExpressionFor(node *ast.UnaryOperatorExpression, lval bool) (*rt.Object, *Unwind) {
	contract.Assertf(!lval || node.Operator == ast.OpDereference, "Only dereference unary ops support l-values")

	// Evaluate the operand and prepare to use it.
	var opand *rt.Object
	var opandloc *location
	if node.Operator == ast.OpAddressof ||
		node.Operator == ast.OpPlusPlus || node.Operator == ast.OpMinusMinus {
		// These operators require an l-value; so we bind the expression a bit differently.
		loc, uw := e.evalLValueExpression(node.Operand)
		if uw != nil {
			return nil, uw
		}
		opand = loc.Obj
		opandloc = &loc
	} else {
		// Otherwise, we just need to evaluate the operand as usual.
		var uw *Unwind
		if opand, uw = e.evalExpression(node.Operand); uw != nil {
			return nil, uw
		}
	}

	// Now switch on the operator and perform its specific operation.
	switch node.Operator {
	case ast.OpDereference:
		// The target is a pointer.  If this is for an l-value, just return it as-is; otherwise, dereference it.
		ptr := opand.PointerValue()
		contract.Assert(ptr != nil)
		if lval {
			return opand, nil
		}
		return ptr.Obj(), nil
	case ast.OpAddressof:
		// The target is an l-value, load its address.
		contract.Assert(opand.PointerValue() != nil)
		return opand, nil
	case ast.OpUnaryPlus:
		// The target is a number; simply fetch it (asserting its value), and + it.
		return e.alloc.NewNumber(+opand.NumberValue()), nil
	case ast.OpUnaryMinus:
		// The target is a number; simply fetch it (asserting its value), and - it.
		return e.alloc.NewNumber(-opand.NumberValue()), nil
	case ast.OpLogicalNot:
		// The target is a boolean; simply fetch it (asserting its value), and ! it.
		return e.alloc.NewBool(!opand.BoolValue()), nil
	case ast.OpBitwiseNot:
		// The target is a number; simply fetch it (asserting its value), and ^ it (similar to C's ~ operator).
		return e.alloc.NewNumber(float64(^int64(opand.NumberValue()))), nil
	case ast.OpPlusPlus:
		// The target is an l-value; we must load it, ++ it, and return the appropriate prefix/postfix value.
		ptr := opand.PointerValue()
		old := ptr.Obj()
		val := old.NumberValue()
		new := e.alloc.NewNumber(val + 1)
		e.evalAssign(node.Operand, *opandloc, new)
		if node.Postfix {
			return old, nil
		}
		return new, nil
	case ast.OpMinusMinus:
		// The target is an l-value; we must load it, -- it, and return the appropriate prefix/postfix value.
		ptr := opand.PointerValue()
		old := ptr.Obj()
		val := old.NumberValue()
		new := e.alloc.NewNumber(val - 1)
		e.evalAssign(node.Operand, *opandloc, new)
		if node.Postfix {
			return old, nil
		}
		return new, nil
	default:
		contract.Failf("Unrecognized unary operator: %v", node.Operator)
		return nil, nil
	}
}

func (e *evaluator) evalBinaryOperatorExpression(node *ast.BinaryOperatorExpression) (*rt.Object, *Unwind) {
	// Evaluate the operands and prepare to use them.  First left, then right.
	var lhs *rt.Object
	var lhsloc *location
	if isBinaryAssignmentOperator(node.Operator) {
		loc, uw := e.evalLValueExpression(node.Left)
		if uw != nil {
			return nil, uw
		}
		lhs = loc.Obj
		lhsloc = &loc
	} else {
		var uw *Unwind
		if lhs, uw = e.evalExpression(node.Left); uw != nil {
			return nil, uw
		}
	}

	// For the logical && and ||, we will only evaluate the rhs it if the lhs was true.
	if node.Operator == ast.OpLogicalAnd || node.Operator == ast.OpLogicalOr {
		if lhs.BoolValue() {
			return e.evalExpression(node.Right)
		}
		return e.alloc.NewBool(false), nil
	}

	// Otherwise, just evaluate the rhs and prepare to evaluate the operator.
	rhs, uw := e.evalExpression(node.Right)
	if uw != nil {
		return nil, uw
	}

	// Switch on operator to perform the operator's effects.
	// TODO: anywhere there is type coercion to/from float64/int64/etc., we should be skeptical.  Because our numeric
	//     type system is float64-based -- i.e., "JSON-like" -- we often find ourselves doing operations on floats that
	//     honestly don't make sense (like shifting, masking, and whatnot).  If there is a type coercion, Golang
	//     (rightfully) doesn't support an operator on numbers of that type.  I suspect we will eventually want to
	//     consider integer types in MuIL, and/or verify that numbers aren't outside of the legal range as part of
	//     verification, and then push the responsibility for presenting valid MuIL with whatever conversions are
	//     necessary back up to the MetaMu compilers (compile-time, runtime, or othwerwise, per the language semantics).
	switch node.Operator {
	// Arithmetic operators
	case ast.OpAdd:
		// If the lhs/rhs are strings, concatenate them; if numbers, + them.
		if lhs.Type() == types.String {
			return e.alloc.NewString(lhs.StringValue() + rhs.StringValue()), nil
		}
		return e.alloc.NewNumber(lhs.NumberValue() + rhs.NumberValue()), nil
	case ast.OpSubtract:
		// Both targets are numbers; fetch them (asserting their types), and - them.
		return e.alloc.NewNumber(lhs.NumberValue() - rhs.NumberValue()), nil
	case ast.OpMultiply:
		// Both targets are numbers; fetch them (asserting their types), and * them.
		return e.alloc.NewNumber(lhs.NumberValue() * rhs.NumberValue()), nil
	case ast.OpDivide:
		// Both targets are numbers; fetch them (asserting their types), and / them.
		return e.alloc.NewNumber(lhs.NumberValue() / rhs.NumberValue()), nil
	case ast.OpRemainder:
		// Both targets are numbers; fetch them (asserting their types), and % them.
		return e.alloc.NewNumber(float64(int64(lhs.NumberValue()) % int64(rhs.NumberValue()))), nil
	case ast.OpExponentiate:
		// Both targets are numbers; fetch them (asserting their types), and raise lhs to rhs's power.
		return e.alloc.NewNumber(math.Pow(lhs.NumberValue(), rhs.NumberValue())), nil

	// Bitwise operators
	// TODO: the ECMAScript specification for bitwise operators is a fair bit more complicated than these; for instance,
	//     shifts mask out all but the least significant 5 bits of the rhs.  If we don't do it here, MuJS should; e.g.
	//     see https://www.ecma-international.org/ecma-262/7.0/#sec-left-shift-operator.
	case ast.OpBitwiseShiftLeft:
		// Both targets are numbers; fetch them (asserting their types), and << them.
		// TODO: consider a verification error if rhs is negative.
		return e.alloc.NewNumber(float64(int64(lhs.NumberValue()) << uint(rhs.NumberValue()))), nil
	case ast.OpBitwiseShiftRight:
		// Both targets are numbers; fetch them (asserting their types), and >> them.
		// TODO: consider a verification error if rhs is negative.
		return e.alloc.NewNumber(float64(int64(lhs.NumberValue()) >> uint(rhs.NumberValue()))), nil
	case ast.OpBitwiseAnd:
		// Both targets are numbers; fetch them (asserting their types), and & them.
		return e.alloc.NewNumber(float64(int64(lhs.NumberValue()) & int64(rhs.NumberValue()))), nil
	case ast.OpBitwiseOr:
		// Both targets are numbers; fetch them (asserting their types), and | them.
		return e.alloc.NewNumber(float64(int64(lhs.NumberValue()) | int64(rhs.NumberValue()))), nil
	case ast.OpBitwiseXor:
		// Both targets are numbers; fetch them (asserting their types), and ^ them.
		return e.alloc.NewNumber(float64(int64(lhs.NumberValue()) ^ int64(rhs.NumberValue()))), nil

	// Assignment operators
	case ast.OpAssign:
		// The target is an l-value; just overwrite its value, and yield the new value as the result.
		e.evalAssign(node.Left, *lhsloc, rhs)
		return rhs, nil
	case ast.OpAssignSum:
		// The target is a numeric l-value; just += rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(ptr.Obj().NumberValue() + rhs.NumberValue())
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignDifference:
		// The target is a numeric l-value; just -= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(ptr.Obj().NumberValue() - rhs.NumberValue())
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignProduct:
		// The target is a numeric l-value; just *= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(ptr.Obj().NumberValue() * rhs.NumberValue())
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignQuotient:
		// The target is a numeric l-value; just /= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(ptr.Obj().NumberValue() / rhs.NumberValue())
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignRemainder:
		// The target is a numeric l-value; just %= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(float64(int64(ptr.Obj().NumberValue()) % int64(rhs.NumberValue())))
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignExponentiation:
		// The target is a numeric l-value; just raise to rhs as a power, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(math.Pow(ptr.Obj().NumberValue(), rhs.NumberValue()))
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignBitwiseShiftLeft:
		// The target is a numeric l-value; just <<= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(float64(int64(ptr.Obj().NumberValue()) << uint(rhs.NumberValue())))
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignBitwiseShiftRight:
		// The target is a numeric l-value; just >>= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(float64(int64(ptr.Obj().NumberValue()) >> uint(rhs.NumberValue())))
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignBitwiseAnd:
		// The target is a numeric l-value; just &= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(float64(int64(ptr.Obj().NumberValue()) & int64(rhs.NumberValue())))
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignBitwiseOr:
		// The target is a numeric l-value; just |= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(float64(int64(ptr.Obj().NumberValue()) | int64(rhs.NumberValue())))
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil
	case ast.OpAssignBitwiseXor:
		// The target is a numeric l-value; just ^= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := e.alloc.NewNumber(float64(int64(ptr.Obj().NumberValue()) ^ int64(rhs.NumberValue())))
		e.evalAssign(node.Left, *lhsloc, val)
		return val, nil

	// Relational operators
	case ast.OpLt:
		// The targets are numbers; just compare them with < and yield the boolean result.
		return e.alloc.NewBool(lhs.NumberValue() < rhs.NumberValue()), nil
	case ast.OpLtEquals:
		// The targets are numbers; just compare them with <= and yield the boolean result.
		return e.alloc.NewBool(lhs.NumberValue() <= rhs.NumberValue()), nil
	case ast.OpGt:
		// The targets are numbers; just compare them with > and yield the boolean result.
		return e.alloc.NewBool(lhs.NumberValue() > rhs.NumberValue()), nil
	case ast.OpGtEquals:
		// The targets are numbers; just compare them with >= and yield the boolean result.
		return e.alloc.NewBool(lhs.NumberValue() >= rhs.NumberValue()), nil
	case ast.OpEquals:
		// Equality checking handles many object types, so defer to a helper for it.
		return e.alloc.NewBool(e.evalBinaryOperatorEquals(lhs, rhs)), nil
	case ast.OpNotEquals:
		// Just return the inverse of what the operator equals function itself returns.
		return e.alloc.NewBool(!e.evalBinaryOperatorEquals(lhs, rhs)), nil

	default:
		contract.Failf("Unrecognized binary operator: %v", node.Operator)
		return nil, nil
	}
}

func isBinaryAssignmentOperator(op ast.BinaryOperator) bool {
	switch op {
	case ast.OpAssign, ast.OpAssignSum, ast.OpAssignDifference, ast.OpAssignProduct, ast.OpAssignQuotient,
		ast.OpAssignRemainder, ast.OpAssignExponentiation, ast.OpAssignBitwiseShiftLeft, ast.OpAssignBitwiseShiftRight,
		ast.OpAssignBitwiseAnd, ast.OpAssignBitwiseOr, ast.OpAssignBitwiseXor:
		return true
	default:
		return false
	}
}

func (e *evaluator) evalAssign(node ast.Node, loc location, val *rt.Object) {
	// Perform the assignment, but make sure to invoke the property assignment hook if necessary.
	ptr := loc.Obj.PointerValue()

	// If the pointer is readonly, however, we will disallow the assignment.
	if ptr.Readonly() {
		e.Diag().Errorf(errors.ErrorIllegalReadonlyLValue.At(node))
	} else {
		if e.hooks != nil {
			e.hooks.OnVariableAssign(loc.This, loc.Name, ptr.Obj(), val)
		}
		ptr.Set(val)
	}
}

func (e *evaluator) evalBinaryOperatorEquals(lhs *rt.Object, rhs *rt.Object) bool {
	if lhs == rhs {
		return true
	}
	if lhs.Type() == types.Bool && rhs.Type() == types.Bool {
		return lhs.BoolValue() == rhs.BoolValue()
	}
	if lhs.Type() == types.Number && rhs.Type() == types.Number {
		return lhs.NumberValue() == rhs.NumberValue()
	}
	if lhs.Type() == types.String && rhs.Type() == types.String {
		return lhs.StringValue() == rhs.StringValue()
	}
	if lhs.Type() == types.Null && rhs.Type() == types.Null {
		return true // all nulls are equal.
	}
	return false
}

func (e *evaluator) evalCastExpression(node *ast.CastExpression) (*rt.Object, *Unwind) {
	// Evaluate the underlying expression.
	obj, uw := e.evalExpression(node.Expression)
	if uw != nil {
		return nil, uw
	}

	// All bad static casts have been rejected, so we now need to check the runtime types.
	from := obj.Type()
	to := e.ctx.RequireType(node)
	if !types.CanConvert(from, to) {
		return nil, NewThrowUnwind(e.NewInvalidCastException(node, from, to))
	}

	return obj, nil
}

func (e *evaluator) evalIsInstExpression(node *ast.IsInstExpression) (*rt.Object, *Unwind) {
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalTypeOfExpression(node *ast.TypeOfExpression) (*rt.Object, *Unwind) {
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalConditionalExpression(node *ast.ConditionalExpression) (*rt.Object, *Unwind) {
	// Evaluate the branches explicitly based on the result of the condition node.
	cond, uw := e.evalExpression(node.Condition)
	if uw != nil {
		return nil, uw
	}
	if cond.BoolValue() {
		return e.evalExpression(node.Consequent)
	}
	return e.evalExpression(node.Alternate)
}

func (e *evaluator) evalSequenceExpression(node *ast.SequenceExpression) (*rt.Object, *Unwind) {
	// Simply walk through the sequence and return the last object.
	var obj *rt.Object
	contract.Assert(len(node.Expressions) > 0)
	for _, expr := range node.Expressions {
		var uw *Unwind
		if obj, uw = e.evalExpression(expr); uw != nil {
			// If the Unwind was non-nil, stop visiting the expressions and propagate it now.
			return nil, uw
		}
	}
	// Return the last expression's object.
	return obj, nil
}
