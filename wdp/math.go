package wdp

import "math"

// Provide a qualitative explanation for what a particular epsilon value means.
func QualEps(eps, p float64) float64 {
	 //    Recommended description:
	 //        If someone believed a user was in the data with probability p,
	 //        then at most after seeing the data they will be qual_eps(eps, p) certain
	 //        (assuming the sensitivity value is correct).
	 //        e.g., for eps=1; p=0.5, they'd go from 50% certainty to at most 73.1% certainty.

	 //    Parameters:
	 //        eps: epsilon value that quantifies "privacy" level of differential privacy.
	 //        p: initial belief that a given user is in the data -- e.g.,:
	 //            0.5 represents complete uncertainty (50/50 chance)
	 //            0.01 represents high certainty the person isn't in the data
	 //            0.99 represents high certainty the person is in the data
	if p > 0 && p < 1 {
        return (math.Exp(eps) * p) / (1 + ((math.Exp(eps) - 1) * p))
	} else {
        return -1
    }
}

func AggregationThreshold(sensitivity int, eps, alpha, propWithin float64) float64 {
	// Same as doAggregate but determines threshold where data is deemed 'too noisy'.
	var rank = alpha / 2
	var lbda = float64(sensitivity) / eps
	// get confidence interval; this is symmetric where `lower bound = noisedX - ci` and `upper bound = noisedX + ci`
	var ci = math.Abs(lbda * math.Log(2*rank))
	return math.Ceil(ci / propWithin)
}

func DoAggregate(noisedX, sensitivity int, eps, alpha, propWithin float64) int {
    // Check whether noisy data X is at least (100 * alpha)% of being within Y% of true value.
    // Doesn't use true value (only noisy value and parameters) so no privacy cost to this.
    // Should identify in advance what threshold -- e.g., 50% probability within 25% of actual value -- in advance
    // to determine whether to keep the data or suppress it until it can be further aggregated so more signal to noise.
    // See a more complete description in the paper below for how to effectively use this data.

    // Based on:
    // * Description: https://arxiv.org/pdf/2009.01265.pdf#section.4
    // * Code: https://github.com/google/differential-privacy/blob/main/java/main/com/google/privacy/differentialprivacy/LaplaceNoise.java#L127

    // Parameters:
    //     noisedX: the count after adding Laplace noise
    //     sensitivity: L1 sensitivity for Laplace
    //     eps: selected epsilon value
    //     alpha: how confident (0.5 = 50%; 0.1 = 90%) should we be that the noisy data is within (100 * prop_within)% of the true data?
    //     prop_within: how close (0.25 = 25%) to actual value should we expect the noisy data to be?
    
    // Divide alpha by 2 because two-tailed
    var rank = alpha / 2
    var lbda = float64(sensitivity) / eps
    // get confidence interval; this is symmetric where `lower bound = noisedX - ci` and `upper bound = noisedX + ci`
    var ci = math.Abs(float64(lbda) * math.Log(2*rank))
    if ci > (propWithin * float64(noisedX)) {
        return 1
    } else {
        return 0
    }
}