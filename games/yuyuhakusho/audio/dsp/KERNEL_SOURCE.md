# Fixed numerical kernel provenance

`kernel.go` contains the 264 integer coefficients of the `Sinc_Generator(0.9,0.55,4.5)` mathematical step-kernel profile used by `blip_buf`. The native source is pinned at Genesis Plus GX commit `60bee4c724859710ce05672239e5b601ad01382e`, `core/sound/blip_buf.c`, SHA-256 `e4ca194284710361b033505aeb1f4283ae8d2e8cdcd2153a598604743f99abaf`.

The source attributes `blip_buf` to Shay Green, copyright 2003–2009, under GNU LGPL version 2.1 or later. This attribution and the source license notice apply to the numerical profile. The Go DSP implementation uses independently written typed arithmetic; it does not include a C implementation, native core, CPU, firmware or native context.

Original notice:

> blip_buf $vers. http://www.slack.net/~ant/
>
> Copyright (C) 2003–2009 Shay Green. This library is free software; you can redistribute it and/or modify it under the terms of the GNU Lesser General Public License as published by the Free Software Foundation; either version 2.1 of the License, or (at your option) any later version.
>
> This library is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU Lesser General Public License for more details.

Source: [blip_buf.c at the pinned commit](https://github.com/libretro/Genesis-Plus-GX/blob/60bee4c724859710ce05672239e5b601ad01382e/core/sound/blip_buf.c). License text: [GNU LGPL 2.1](https://www.gnu.org/licenses/old-licenses/lgpl-2.1.html).

Tests retain both these exact source coefficients and independent native PCM/state oracles. Coefficient identity alone is not used as proof that the complete renderer is correct.
