import { createRequire } from "node:module";
import path from "node:path";

const require = createRequire(new URL("../package.json", import.meta.url));
const ts = require("typescript");
const INTEGER = 1;
const FLOAT = 2;
const OTHER = 4;
const UNKNOWN = 8;
const NONFINITE = 16;

export function inspectNumericComparisons(file, emit) {
  const options = { noLib: true, noResolve: true, allowJs: true, checkJs: true, target: ts.ScriptTarget.Latest, module: ts.ModuleKind.ESNext };
  const host = ts.createCompilerHost(options);
  const selected = path.resolve(file.fileName);
  host.getSourceFile = name => path.resolve(name) === selected ? file : undefined;
  host.fileExists = name => path.resolve(name) === selected;
  host.readFile = name => path.resolve(name) === selected ? file.text : undefined;
  host.writeFile = () => {};
  const checker = ts.createProgram([file.fileName], options, host).getTypeChecker();
  const symbolKeys = new Map();
  const writes = new Map();
  const mutations = new Map();
  let nextKey = 1;
  const join = values => values.reduce((result, value) => result | value, 0) || UNKNOWN;
  const symbolKey = node => {
    const symbol = checker.getSymbolAtLocation(node);
    if (!symbol) return node.text ?? node.getText(file);
    if (!symbolKeys.has(symbol)) symbolKeys.set(symbol, `symbol-${nextKey++}`);
    return symbolKeys.get(symbol);
  };
  function key(node) {
    if (ts.isIdentifier(node)) return symbolKey(node);
    if (ts.isPropertyAccessExpression(node)) return `${key(node.expression)}.${node.name.text}`;
    if (ts.isElementAccessExpression(node)) return `${key(node.expression)}[${node.argumentExpression?.getText(file) ?? ""}]`;
    if (ts.isParenthesizedExpression(node) || ts.isNonNullExpression(node)) return key(node.expression);
    return node.getText(file);
  }
  function remember(node, value, position, operator) {
    const name = key(node);
    if (!writes.has(name)) writes.set(name, []);
    const write = { target: node, value, position, at: node.getStart(file), operator };
    writes.get(name).push(write);
    return write;
  }
  function mutate(node, write) {
    const name = key(node);
    if (!mutations.has(name)) mutations.set(name, []);
    mutations.get(name).push(write);
  }
  function collect(node) {
    if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.initializer) remember(node.name, node.initializer, node.end);
    if (ts.isBinaryExpression(node) && node.operatorToken.kind >= ts.SyntaxKind.FirstAssignment && node.operatorToken.kind <= ts.SyntaxKind.LastAssignment) {
      const write = remember(node.left, node.right, node.end, node.operatorToken.kind);
      if (ts.isElementAccessExpression(node.left)) mutate(node.left.expression, write);
    }
    if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && ["push", "unshift", "splice", "fill"].includes(node.expression.name.text)) {
      const values = node.expression.name.text === "splice" ? node.arguments.slice(2) : node.expression.name.text === "fill" ? node.arguments.slice(0, 1) : node.arguments;
      for (const value of values) mutate(node.expression.expression, { value, position: node.end, at: node.getStart(file) });
    }
    ts.forEachChild(node, collect);
  }
  collect(file);
  const declaration = node => checker.getSymbolAtLocation(node)?.valueDeclaration ?? checker.getSymbolAtLocation(node)?.declarations?.[0];
  const lastWrite = (node, at) => writes.get(key(node))?.filter(write => write.position < at).at(-1);
  const mergeFacts = (left, right) => {
    const result = new Map(left);
    for (const [name, value] of right) result.set(name, result.get(name) === INTEGER && value === FLOAT ? INTEGER : value);
    return result;
  };
  function nativeName(node, depth = 0) {
    if (!node || depth > 24) return "";
    if (ts.isPropertyAccessExpression(node) && ts.isIdentifier(node.expression) && ["Math", "Number", "Array"].includes(node.expression.text) && !declaration(node.expression)) return `${node.expression.text}.${node.name.text}`;
    if (ts.isPropertyAccessExpression(node) && ["call", "apply"].includes(node.name.text)) {
      const called = nativeName(node.expression, depth + 1);
      if (["Math.min", "Math.max"].includes(called)) return called;
    }
    if (ts.isElementAccessExpression(node) && ts.isIdentifier(node.expression) && node.argumentExpression && ts.isStringLiteral(node.argumentExpression) && ["Math", "Number"].includes(node.expression.text) && !declaration(node.expression)) return `${node.expression.text}.${node.argumentExpression.text}`;
    if (ts.isIdentifier(node)) {
      const declared = declaration(node);
      if (declared && ts.isVariableDeclaration(declared) && declared.initializer) return nativeName(declared.initializer, depth + 1);
      if (!declared) return node.text;
    }
    if (ts.isParenthesizedExpression(node)) return nativeName(node.expression, depth + 1);
    return "";
  }
  function facts(expression, truth = true) {
    if (ts.isParenthesizedExpression(expression)) return facts(expression.expression, truth);
    if (ts.isPrefixUnaryExpression(expression) && expression.operator === ts.SyntaxKind.ExclamationToken) return facts(expression.operand, !truth);
    if (ts.isCallExpression(expression) && truth) {
      if (["Number.isInteger", "Number.isSafeInteger"].includes(nativeName(expression.expression)) && expression.arguments[0]) return new Map([[key(expression.arguments[0]), INTEGER]]);
      if (ts.isPropertyAccessExpression(expression.expression) && expression.expression.name.text === "every" && expression.arguments[0] && ["Number.isInteger", "Number.isSafeInteger"].includes(nativeName(expression.arguments[0]))) return new Map([[`${key(expression.expression.expression)}[]`, INTEGER]]);
    }
    if (ts.isBinaryExpression(expression)) {
      const operation = expression.operatorToken.kind;
      if (operation === ts.SyntaxKind.AmpersandAmpersandToken && truth || operation === ts.SyntaxKind.BarBarToken && !truth) return mergeFacts(facts(expression.left, truth), facts(expression.right, truth));
      if (operation === ts.SyntaxKind.BarBarToken && truth || operation === ts.SyntaxKind.AmpersandAmpersandToken && !truth) {
        const left = facts(expression.left, truth), right = facts(expression.right, truth);
        return new Map([...left].filter(([name]) => right.has(name)).map(([name, value]) => [name, value | right.get(name)]));
      }
      const equals = [ts.SyntaxKind.EqualsEqualsToken, ts.SyntaxKind.EqualsEqualsEqualsToken].includes(operation);
      const differs = [ts.SyntaxKind.ExclamationEqualsToken, ts.SyntaxKind.ExclamationEqualsEqualsToken].includes(operation);
      if ((equals || differs) && truth === equals) {
        for (const [type, label] of [[expression.left, expression.right], [expression.right, expression.left]]) if (ts.isTypeOfExpression(type) && ts.isStringLiteral(label)) return new Map([[key(type.expression), label.text === "number" ? FLOAT : label.text === "bigint" ? INTEGER : OTHER]]);
      }
    }
    return new Map();
  }
  function terminates(node) {
    if (ts.isReturnStatement(node) || ts.isThrowStatement(node)) return true;
    if (ts.isBlock(node)) return !!node.statements.length && terminates(node.statements.at(-1));
    return false;
  }
  function guarded(node) {
    const result = new Map();
    const add = (expression, truth = true) => {
      for (const [name, value] of facts(expression, truth)) if (!result.has(name) || expression.end >= result.get(name).position) result.set(name, { kind: value, position: expression.end });
    };
    for (let child = node, parent = node.parent; parent; child = parent, parent = parent.parent) {
      if (ts.isBinaryExpression(parent) && child === parent.right && parent.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken) add(parent.left);
      if (ts.isBinaryExpression(parent) && child === parent.right && parent.operatorToken.kind === ts.SyntaxKind.BarBarToken) add(parent.left, false);
      if (ts.isIfStatement(parent) && child === parent.thenStatement) add(parent.expression);
      if (ts.isIfStatement(parent) && child === parent.elseStatement) add(parent.expression, false);
      if (ts.isConditionalExpression(parent) && child === parent.whenTrue) add(parent.condition);
      if (ts.isConditionalExpression(parent) && child === parent.whenFalse) add(parent.condition, false);
      if (ts.isBlock(parent) || ts.isSourceFile(parent)) for (const statement of parent.statements) {
        if (statement.getStart(file) >= child.getStart(file)) break;
        if (ts.isIfStatement(statement) && !statement.elseStatement && terminates(statement.thenStatement)) add(statement.expression, false);
      }
    }
    return result;
  }
  function written(write, environment, seen) {
    if (seen.has(write)) return UNKNOWN;
    const next = new Set([...seen, write]);
    if (write.operator === ts.SyntaxKind.SlashEqualsToken) return FLOAT;
    if ([ts.SyntaxKind.BarEqualsToken, ts.SyntaxKind.AmpersandEqualsToken, ts.SyntaxKind.CaretEqualsToken, ts.SyntaxKind.LessThanLessThanEqualsToken, ts.SyntaxKind.GreaterThanGreaterThanEqualsToken, ts.SyntaxKind.GreaterThanGreaterThanGreaterThanEqualsToken].includes(write.operator)) return INTEGER;
    const value = numeric(write.value, environment, next, write.at);
    if (write.operator && write.operator !== ts.SyntaxKind.EqualsToken) return value | numeric(write.target, environment, next, write.at);
    return value;
  }
  function typeKind(type) {
    if (!type) return UNKNOWN;
    if (type.isUnion()) return join(type.types.map(typeKind));
    if (type.flags & ts.TypeFlags.NumberLiteral) return Number.isInteger(type.value) ? INTEGER : FLOAT;
    if (type.flags & ts.TypeFlags.NumberLike) return FLOAT;
    if (type.flags & ts.TypeFlags.BigIntLike) return INTEGER;
    if (type.flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown | ts.TypeFlags.TypeParameter)) return UNKNOWN;
    return OTHER;
  }
  function propertyValue(node) {
    if (!ts.isPropertyAccessExpression(node)) return;
    const owner = lastWrite(node.expression, node.getStart(file))?.value;
    if (owner && ts.isObjectLiteralExpression(owner)) return owner.properties.find(property => ts.isPropertyAssignment(property) && property.name.getText(file).replaceAll(/["']/g, "") === node.name.text)?.initializer;
  }
  function callable(node, depth = 0) {
    if (!node || depth > 24) return;
    if (ts.isArrowFunction(node) || ts.isFunctionExpression(node) || ts.isFunctionDeclaration(node) || ts.isMethodDeclaration(node)) return node;
    const declared = declaration(ts.isPropertyAccessExpression(node) ? node.name : node);
    if (declared && ts.isVariableDeclaration(declared)) return callable(declared.initializer, depth + 1);
    if (declared && (ts.isFunctionDeclaration(declared) || ts.isMethodDeclaration(declared))) return declared;
  }
  function returned(fn, args, environment, seen) {
    if (!fn?.body) return UNKNOWN;
    const next = new Map(environment);
    fn.parameters.forEach((parameter, index) => next.set(key(parameter.name), args[index] ?? typeKind(checker.getTypeAtLocation(parameter))));
    if (!ts.isBlock(fn.body)) return numeric(fn.body, next, seen);
    const values = [];
    function visit(node) {
      if (ts.isReturnStatement(node)) values.push(node.expression ? numeric(node.expression, next, seen) : OTHER);
      else if (!ts.isFunctionLike(node)) ts.forEachChild(node, visit);
    }
    ts.forEachChild(fn.body, visit);
    return join(values);
  }
  function elementKind(node, environment = new Map(), seen = new Set()) {
    if (!node || seen.size > 48) return UNKNOWN;
    const changes = mutations.get(key(node))?.filter(write => write.position < node.getStart(file)) ?? [];
    const proof = guarded(node).get(`${key(node)}[]`);
    if (proof?.kind === INTEGER && !changes.some(write => write.position > proof.position && written(write, environment, seen) & (FLOAT | UNKNOWN))) return INTEGER;
    let result;
    if (ts.isArrayLiteralExpression(node)) result = join(node.elements.map(element => numeric(element, environment, seen)));
    const write = lastWrite(node, node.getStart(file));
    if (write && !seen.has(write.value)) result = elementKind(write.value, environment, new Set([...seen, write.value]));
    const property = propertyValue(node);
    if (property) result = elementKind(property, environment, seen);
    if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "map") result = returned(callable(node.arguments[0]), [elementKind(node.expression.expression, environment, seen), INTEGER], environment, seen);
    const declared = declaration(ts.isPropertyAccessExpression(node) ? node.name : node);
    let type = declared?.type;
    if (type && ts.isTypeOperatorNode(type)) type = type.type;
    if (result === undefined && type && ts.isArrayTypeNode(type)) result = typeKind(checker.getTypeFromTypeNode(type.elementType));
    if (result === undefined) result = typeKind(checker.getTypeAtLocation(node).getNumberIndexType?.());
    return result | changes.reduce((value, change) => value | written(change, environment, seen), 0);
  }
  function numeric(node, environment = new Map(), seen = new Set(), at = node?.getStart(file)) {
    if (!node || seen.size > 48) return UNKNOWN;
    const guardedValue = guarded(node).get(key(node));
    const write = lastWrite(node, at);
    if (guardedValue !== undefined && (!write || write.position <= guardedValue.position)) return guardedValue.kind;
    if (write) return written(write, environment, seen);
    if (environment.has(key(node))) return environment.get(key(node));
    if (ts.isNumericLiteral(node)) return Number.isInteger(Number(node.text)) ? INTEGER : FLOAT;
    if (ts.isBigIntLiteral(node)) return INTEGER;
    if (ts.isStringLiteralLike(node) || [ts.SyntaxKind.TrueKeyword, ts.SyntaxKind.FalseKeyword, ts.SyntaxKind.NullKeyword].includes(node.kind) || ts.isTypeOfExpression(node)) return OTHER;
    if (ts.isParenthesizedExpression(node) || ts.isNonNullExpression(node)) return numeric(node.expression, environment, seen, at);
    if (ts.isAsExpression(node) || ts.isTypeAssertionExpression(node)) {
      const value = numeric(node.expression, environment, seen, at);
      return value === UNKNOWN ? typeKind(checker.getTypeFromTypeNode(node.type)) : value;
    }
    if (ts.isPrefixUnaryExpression(node)) return node.operator === ts.SyntaxKind.ExclamationToken ? OTHER : numeric(node.operand, environment, seen, at);
    if (ts.isConditionalExpression(node)) return numeric(node.whenTrue, environment, seen) | numeric(node.whenFalse, environment, seen);
    if (ts.isElementAccessExpression(node)) return elementKind(node.expression, environment, seen);
    if (ts.isIdentifier(node) || ts.isPropertyAccessExpression(node)) {
      if (ts.isIdentifier(node) && ["undefined", "NaN", "Infinity"].includes(node.text)) return node.text === "undefined" ? OTHER : FLOAT | NONFINITE;
      if (["Number.NaN", "Number.POSITIVE_INFINITY", "Number.NEGATIVE_INFINITY"].includes(nativeName(node))) return FLOAT | NONFINITE;
      if (nativeName(node) === "Number.EPSILON") return FLOAT;
      const property = propertyValue(node);
      if (property) return numeric(property, environment, seen);
      return typeKind(checker.getTypeAtLocation(node));
    }
    if (ts.isBinaryExpression(node)) {
      const operation = node.operatorToken.kind;
      if ([ts.SyntaxKind.BarToken, ts.SyntaxKind.AmpersandToken, ts.SyntaxKind.CaretToken, ts.SyntaxKind.LessThanLessThanToken, ts.SyntaxKind.GreaterThanGreaterThanToken, ts.SyntaxKind.GreaterThanGreaterThanGreaterThanToken].includes(operation)) return INTEGER;
      if (operation === ts.SyntaxKind.SlashToken) return FLOAT;
      if ([ts.SyntaxKind.PlusToken, ts.SyntaxKind.MinusToken, ts.SyntaxKind.AsteriskToken, ts.SyntaxKind.PercentToken, ts.SyntaxKind.AsteriskAsteriskToken].includes(operation)) {
        const value = numeric(node.left, environment, seen, at) | numeric(node.right, environment, seen, at);
        return operation !== ts.SyntaxKind.PlusToken && value & UNKNOWN ? value | FLOAT : value;
      }
      return OTHER;
    }
    if (ts.isCallExpression(node)) {
      const name = nativeName(node.expression);
      if (["Math.round", "Math.trunc", "Math.floor", "Math.ceil", "parseInt", "Number.parseInt", "BigInt"].includes(name)) return node.arguments.some(argument => numeric(argument, environment, seen) & NONFINITE) ? FLOAT | NONFINITE : INTEGER;
      if (["Math.min", "Math.max", "Math.abs"].includes(name)) return join(node.arguments.map(argument => numeric(argument, environment, seen)));
      if (name.startsWith("Math.") || ["Number", "parseFloat", "Number.parseFloat"].includes(name)) return FLOAT;
      const fn = callable(node.expression);
      if (fn?.body && !seen.has(fn)) return returned(fn, node.arguments.map(argument => numeric(argument, environment, seen)), environment, new Set([...seen, fn]));
      return typeKind(checker.getTypeAtLocation(node));
    }
    return typeKind(checker.getTypeAtLocation(node));
  }
  function extremaArguments(node) {
    if (ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "call") return node.arguments.slice(1).map(argument => numeric(argument));
    if (ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "apply") return [node.arguments[1] ? elementKind(node.arguments[1]) : UNKNOWN];
    return node.arguments.map(argument => numeric(argument));
  }
  const sentinel = node => node.kind === ts.SyntaxKind.NullKeyword || ts.isIdentifier(node) && node.text === "undefined" || ts.isVoidExpression(node);
  function visit(node) {
    if (ts.isBinaryExpression(node) && [ts.SyntaxKind.LessThanToken, ts.SyntaxKind.LessThanEqualsToken, ts.SyntaxKind.GreaterThanToken, ts.SyntaxKind.GreaterThanEqualsToken, ts.SyntaxKind.EqualsEqualsToken, ts.SyntaxKind.EqualsEqualsEqualsToken, ts.SyntaxKind.ExclamationEqualsToken, ts.SyntaxKind.ExclamationEqualsEqualsToken].includes(node.operatorToken.kind)) {
      const left = numeric(node.left), right = numeric(node.right);
      if ((left & FLOAT || right & FLOAT || left & UNKNOWN && right & INTEGER || right & UNKNOWN && left & INTEGER) && !sentinel(node.left) && !sentinel(node.right)) emit("integer-business-comparison", node);
    }
    if (ts.isCallExpression(node)) {
      const name = nativeName(node.expression);
      if (["Math.min", "Math.max"].includes(name) && extremaArguments(node).some(kind => kind !== INTEGER)) emit("integer-business-comparison", node);
      if (ts.isPropertyAccessExpression(node.expression)) {
        const method = node.expression.name.text;
        if (["sort", "toSorted"].includes(method)) {
          const elements = elementKind(node.expression.expression);
          const comparator = callable(node.arguments[0]);
          const result = comparator ? returned(comparator, [elements, elements], new Map(), new Set()) : elements;
          if (result & FLOAT) emit("integer-business-comparison", node);
        }
        if (["includes", "indexOf", "lastIndexOf"].includes(method)) {
          const elements = elementKind(node.expression.expression), requested = node.arguments[0] ? numeric(node.arguments[0]) : OTHER;
          if (elements & FLOAT || requested & FLOAT || elements & UNKNOWN && requested & INTEGER) emit("integer-business-comparison", node);
        }
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(file);
}
